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

package client

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
	"github.com/philips-software/go-dip-api/iam"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1 "github.com/loafoe/provider-hsdp/apis/iam/v1"
	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	"github.com/loafoe/provider-hsdp/internal/clients/dip"
	"github.com/loafoe/provider-hsdp/internal/util"
)

const (
	errNotClient    = "managed resource is not a Client"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create DIP client"
	errGetPassword  = "cannot get password from secret"
)

// Setup adds a controller that reconciles Client managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.ClientGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.ClientGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.Client{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
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

	return &external{client: dipClient, kube: c.kube, namespace: m.GetNamespace()}, nil
}

type external struct {
	client    *dip.Client
	kube      client.Client
	namespace string
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*iamv1.Client)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClient)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	appClient, resp, err := e.client.IAM.Clients.GetClientByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get client")
	}
	if appClient == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &appClient.ID
	cr.Status.AtProvider.ClientID = &appClient.ClientID
	cr.Status.AtProvider.Disabled = &appClient.Disabled
	cr.Status.AtProvider.Type = util.StringPtrOrNil(appClient.Type)
	cr.Status.AtProvider.Description = util.StringPtrOrNil(appClient.Description)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, appClient),
	}, nil
}

func (e *external) isUpToDate(cr *iamv1.Client, appClient *iam.ApplicationClient) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != appClient.Name {
		return false
	}
	if fp.Description != appClient.Description {
		return false
	}
	return true
}

func (e *external) getPassword(ctx context.Context, ref xpv1.SecretKeySelector, namespace string) (string, error) {
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
		key = "password"
	}

	password, ok := secret.Data[key]
	if !ok {
		return "", errors.Errorf("secret %s/%s does not have key %s", nn.Namespace, nn.Name, key)
	}

	return string(password), nil
}

func applyOptionalFields(appClient *iam.ApplicationClient, fp *iamv1.ClientParameters) {
	if fp.ApplicationID != nil {
		appClient.ApplicationID = *fp.ApplicationID
	}
	if fp.RedirectionURIs != nil {
		appClient.RedirectionURIs = fp.RedirectionURIs
	}
	if fp.ResponseTypes != nil {
		appClient.ResponseTypes = fp.ResponseTypes
	}
	if fp.Scopes != nil {
		appClient.Scopes = fp.Scopes
	}
	if fp.DefaultScopes != nil {
		appClient.DefaultScopes = fp.DefaultScopes
	}
	if fp.ConsentImplied != nil {
		appClient.ConsentImplied = *fp.ConsentImplied
	}
	if fp.AccessTokenLifetime != nil {
		appClient.AccessTokenLifetime = int(*fp.AccessTokenLifetime)
	}
	if fp.RefreshTokenLifetime != nil {
		appClient.RefreshTokenLifetime = int(*fp.RefreshTokenLifetime)
	}
	if fp.IDTokenLifetime != nil {
		appClient.IDTokenLifetime = int(*fp.IDTokenLifetime)
	}
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Client)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClient)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	password, err := e.getPassword(ctx, fp.PasswordSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetPassword)
	}

	appClient := iam.ApplicationClient{
		Name:              fp.Name,
		Type:              fp.Type,
		ClientID:          fp.ClientID,
		Password:          password,
		Description:       fp.Description,
		GlobalReferenceID: fp.GlobalReferenceID,
	}

	applyOptionalFields(&appClient, &fp)

	created, _, err := e.client.IAM.Clients.CreateClient(appClient)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create client")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.Client)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClient)
	}

	fp := cr.Spec.ForProvider

	appClient := iam.ApplicationClient{
		ID:                meta.GetExternalName(cr),
		Name:              fp.Name,
		Description:       fp.Description,
		GlobalReferenceID: fp.GlobalReferenceID,
	}

	applyOptionalFields(&appClient, &fp)

	_, _, err := e.client.IAM.Clients.UpdateClient(appClient)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update client")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.Client)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClient)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.Clients.DeleteClient(iam.ApplicationClient{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete client")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
