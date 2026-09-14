/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authenticationmethod

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/philips-software/go-dip-api/connect/mdm"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mdmv1 "github.com/loafoe/provider-hsdp/apis/mdm/v1"
	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	"github.com/loafoe/provider-hsdp/internal/clients/dip"
	"github.com/loafoe/provider-hsdp/internal/util"
)

const (
	errNotAuthenticationMethod = "managed resource is not an MDM AuthenticationMethod"
	errTrackPCUsage            = "cannot track ProviderConfig usage"
	errGetPC                   = "cannot get ProviderConfig"
	errGetCreds                = "cannot get credentials"
	errNewClient               = "cannot create DIP client"
	errGetPassword             = "cannot get password from secret"
	errGetClientSecret         = "cannot get client secret from secret"
)

// Setup adds a controller that reconciles MDM AuthenticationMethod managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(mdmv1.AuthenticationMethodGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apismv1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	// DIP assigns the external name (a GUID) at creation time, so disable the
	// default NameAsExternalName initializer. Otherwise the external name is
	// briefly set to the CR name, which cross-resource reference resolvers can
	// cache before the real GUID is known.
	opts = append(opts, managed.WithInitializers())

	r := managed.NewReconciler(mgr, resource.ManagedKind(mdmv1.AuthenticationMethodGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&mdmv1.AuthenticationMethod{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*mdmv1.AuthenticationMethod)
	if !ok {
		return nil, errors.New(errNotAuthenticationMethod)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	m := mg.(resource.ModernManaged)

	pcSpec, pcKey, err := util.ResolveProviderConfig(ctx, c.kube, m)
	if err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	secretData, err := resource.CommonCredentialExtractor(ctx, pcSpec.Credentials.Source, c.kube,
		xpv1.CommonCredentialSelectors{SecretRef: pcSpec.Credentials.SecretRef})
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	cfg, err := dip.ConfigFromSecret(pcSpec.Region, pcSpec.Environment, secretData)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	dipClient, err := dip.Cache.Get(pcKey, cfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	if dipClient.MDM == nil {
		return nil, errors.New("MDM is not available for the configured region/environment")
	}

	return &external{client: dipClient, kube: c.kube, namespace: m.GetNamespace()}, nil
}

type external struct {
	client    *dip.Client
	kube      client.Client
	namespace string
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*mdmv1.AuthenticationMethod)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotAuthenticationMethod)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	am, resp, err := e.client.MDM.AuthenticationMethods.GetByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get MDM authentication method")
	}
	if am == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &am.ID
	cr.Status.AtProvider.Description = util.StringPtrOrNil(am.Description)
	cr.Status.AtProvider.AuthURL = util.StringPtrOrNil(am.AuthURL)
	cr.Status.AtProvider.AuthMethod = util.StringPtrOrNil(am.AuthMethod)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, am),
	}, nil
}

func (e *external) isUpToDate(cr *mdmv1.AuthenticationMethod, am *mdm.AuthenticationMethod) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != am.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != am.Description {
		return false
	}
	if fp.LoginName != am.LoginName {
		return false
	}
	if fp.ClientID != am.ClientID {
		return false
	}
	return true
}

func (e *external) getSecretValue(ctx context.Context, ref xpv1.SecretKeySelector, namespace string) (string, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}
	if nn.Namespace == "" {
		nn.Namespace = namespace
	}

	secret := &corev1.Secret{}
	if err := e.kube.Get(ctx, nn, secret); err != nil {
		return "", errors.Wrap(err, "cannot get secret")
	}

	key := ref.Key
	if key == "" {
		return "", errors.Errorf("secret key is required")
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", errors.Errorf("secret %s/%s does not have key %s", nn.Namespace, nn.Name, key)
	}

	return string(value), nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*mdmv1.AuthenticationMethod)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotAuthenticationMethod)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	password, err := e.getSecretValue(ctx, fp.PasswordSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetPassword)
	}

	clientSecret, err := e.getSecretValue(ctx, fp.ClientSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetClientSecret)
	}

	am := mdm.AuthenticationMethod{
		ResourceType: "AuthenticationMethod",
		Name:         fp.Name,
		LoginName:    fp.LoginName,
		Password:     password,
		ClientID:     fp.ClientID,
		ClientSecret: clientSecret,
	}

	if fp.Description != nil {
		am.Description = *fp.Description
	}
	if fp.AuthURL != nil {
		am.AuthURL = *fp.AuthURL
	}
	if fp.AuthMethod != nil {
		am.AuthMethod = *fp.AuthMethod
	}
	if fp.APIVersion != nil {
		am.APIVersion = *fp.APIVersion
	}
	if fp.OrganizationID != nil {
		am.OrganizationGuid = &mdm.Identifier{Value: *fp.OrganizationID}
	}

	created, _, err := e.client.MDM.AuthenticationMethods.Create(am)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create MDM authentication method")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*mdmv1.AuthenticationMethod)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotAuthenticationMethod)
	}

	fp := cr.Spec.ForProvider

	password, err := e.getSecretValue(ctx, fp.PasswordSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetPassword)
	}

	clientSecret, err := e.getSecretValue(ctx, fp.ClientSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetClientSecret)
	}

	am := mdm.AuthenticationMethod{
		ResourceType: "AuthenticationMethod",
		ID:           meta.GetExternalName(cr),
		Name:         fp.Name,
		LoginName:    fp.LoginName,
		Password:     password,
		ClientID:     fp.ClientID,
		ClientSecret: clientSecret,
	}

	if fp.Description != nil {
		am.Description = *fp.Description
	}
	if fp.AuthURL != nil {
		am.AuthURL = *fp.AuthURL
	}
	if fp.AuthMethod != nil {
		am.AuthMethod = *fp.AuthMethod
	}
	if fp.APIVersion != nil {
		am.APIVersion = *fp.APIVersion
	}
	if fp.OrganizationID != nil {
		am.OrganizationGuid = &mdm.Identifier{Value: *fp.OrganizationID}
	}

	_, _, err = e.client.MDM.AuthenticationMethods.Update(am)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update MDM authentication method")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*mdmv1.AuthenticationMethod)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotAuthenticationMethod)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.MDM.AuthenticationMethods.Delete(mdm.AuthenticationMethod{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete MDM authentication method")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
