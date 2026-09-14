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

package orgconfiguration

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
	"github.com/philips-software/go-dip-api/connect/provisioning"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	provisioningv1 "github.com/loafoe/provider-hsdp/apis/provisioning/v1"
	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	"github.com/loafoe/provider-hsdp/internal/clients/dip"
	"github.com/loafoe/provider-hsdp/internal/util"
)

const (
	errNotOrgConfiguration = "managed resource is not an OrgConfiguration"
	errTrackPCUsage        = "cannot track ProviderConfig usage"
	errGetPC               = "cannot get ProviderConfig"
	errGetCreds            = "cannot get credentials"
	errNewClient           = "cannot create DIP client"
	errGetSecret           = "cannot get secret"
)

// Setup adds a controller that reconciles OrgConfiguration managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(provisioningv1.OrgConfigurationGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(provisioningv1.OrgConfigurationGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&provisioningv1.OrgConfiguration{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*provisioningv1.OrgConfiguration)
	if !ok {
		return nil, errors.New(errNotOrgConfiguration)
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

	if dipClient.Provisioning == nil {
		return nil, errors.New("provisioning is not available for the configured region/environment")
	}

	return &external{client: dipClient, kube: c.kube, namespace: m.GetNamespace()}, nil
}

type external struct {
	client    *dip.Client
	kube      client.Client
	namespace string
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*provisioningv1.OrgConfiguration)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotOrgConfiguration)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	orgConfig, resp, err := e.client.Provisioning.OrgConfigurationsService.GetOrganizationConfigurationByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get organization configuration")
	}
	if orgConfig == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &orgConfig.ID
	if orgConfig.Meta != nil {
		cr.Status.AtProvider.VersionID = &orgConfig.Meta.VersionID
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, orgConfig),
	}, nil
}

func (e *external) isUpToDate(cr *provisioningv1.OrgConfiguration, orgConfig *provisioning.OrgConfiguration) bool {
	fp := cr.Spec.ForProvider

	if fp.OrganizationID != orgConfig.OrganizationGuid {
		return false
	}
	if fp.ServiceAccount.ServiceAccountID != orgConfig.ServiceAccount.ServiceAccountId {
		return false
	}
	if fp.BootstrapSignature.Algorithm != orgConfig.BootstrapSignature.Algorithm {
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
	cr, ok := mg.(*provisioningv1.OrgConfiguration)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotOrgConfiguration)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	// Get service account key from secret
	serviceAccountKey, err := e.getSecretValue(ctx, fp.ServiceAccount.ServiceAccountKeySecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetSecret)
	}

	// Get public key from secret
	publicKey, err := e.getSecretValue(ctx, fp.BootstrapSignature.PublicKeySecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetSecret)
	}

	orgConfig := provisioning.OrgConfiguration{
		ResourceType:     "OrgConfiguration",
		OrganizationGuid: fp.OrganizationID,
		ServiceAccount: provisioning.ServiceAccount{
			ServiceAccountId:  fp.ServiceAccount.ServiceAccountID,
			ServiceAccountKey: serviceAccountKey,
		},
		BootstrapSignature: provisioning.BootstrapSignature{
			PublicKey: publicKey,
			Algorithm: fp.BootstrapSignature.Algorithm,
		},
	}

	// Add optional config
	if fp.BootstrapSignature.Config != nil {
		orgConfig.BootstrapSignature.Config = provisioning.BootStrapSignatureConfig{}
		if fp.BootstrapSignature.Config.Type != nil {
			orgConfig.BootstrapSignature.Config.Type = *fp.BootstrapSignature.Config.Type
		}
		if fp.BootstrapSignature.Config.Padding != nil {
			orgConfig.BootstrapSignature.Config.Padding = *fp.BootstrapSignature.Config.Padding
		}
		if fp.BootstrapSignature.Config.SaltLength != nil {
			orgConfig.BootstrapSignature.Config.SaltLength = *fp.BootstrapSignature.Config.SaltLength
		}
	}

	created, _, err := e.client.Provisioning.OrgConfigurationsService.CreateOrganizationConfiguration(orgConfig)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create organization configuration")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*provisioningv1.OrgConfiguration)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotOrgConfiguration)
	}

	fp := cr.Spec.ForProvider

	// Get service account key from secret
	serviceAccountKey, err := e.getSecretValue(ctx, fp.ServiceAccount.ServiceAccountKeySecretRef, e.namespace)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetSecret)
	}

	// Get public key from secret
	publicKey, err := e.getSecretValue(ctx, fp.BootstrapSignature.PublicKeySecretRef, e.namespace)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetSecret)
	}

	orgConfig := provisioning.OrgConfiguration{
		ResourceType:     "OrgConfiguration",
		ID:               meta.GetExternalName(cr),
		OrganizationGuid: fp.OrganizationID,
		ServiceAccount: provisioning.ServiceAccount{
			ServiceAccountId:  fp.ServiceAccount.ServiceAccountID,
			ServiceAccountKey: serviceAccountKey,
		},
		BootstrapSignature: provisioning.BootstrapSignature{
			PublicKey: publicKey,
			Algorithm: fp.BootstrapSignature.Algorithm,
		},
	}

	// Add optional config
	if fp.BootstrapSignature.Config != nil {
		orgConfig.BootstrapSignature.Config = provisioning.BootStrapSignatureConfig{}
		if fp.BootstrapSignature.Config.Type != nil {
			orgConfig.BootstrapSignature.Config.Type = *fp.BootstrapSignature.Config.Type
		}
		if fp.BootstrapSignature.Config.Padding != nil {
			orgConfig.BootstrapSignature.Config.Padding = *fp.BootstrapSignature.Config.Padding
		}
		if fp.BootstrapSignature.Config.SaltLength != nil {
			orgConfig.BootstrapSignature.Config.SaltLength = *fp.BootstrapSignature.Config.SaltLength
		}
	}

	_, _, err = e.client.Provisioning.OrgConfigurationsService.UpdateOrganizationConfiguration(orgConfig)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update organization configuration")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*provisioningv1.OrgConfiguration)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotOrgConfiguration)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.Provisioning.OrgConfigurationsService.DeleteOrganizationConfiguration(provisioning.OrgConfiguration{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete organization configuration")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
