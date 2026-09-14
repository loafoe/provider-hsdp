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

package service

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iamv1 "github.com/loafoe/provider-hsdp/apis/iam/v1"
	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	"github.com/loafoe/provider-hsdp/internal/clients/dip"
	"github.com/loafoe/provider-hsdp/internal/util"
)

const (
	errNotService   = "managed resource is not a Service"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create DIP client"
)

// Setup adds a controller that reconciles Service managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.ServiceGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.ServiceGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.Service{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return nil, errors.New(errNotService)
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

	return &external{client: dipClient}, nil
}

type external struct {
	client *dip.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotService)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	service, resp, err := e.client.IAM.Services.GetServiceByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get service")
	}
	if service == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &service.ID
	cr.Status.AtProvider.ServiceID = &service.ServiceID
	cr.Status.AtProvider.Description = util.StringPtrOrNil(service.Description)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, service),
	}, nil
}

func (e *external) isUpToDate(cr *iamv1.Service, service *iam.Service) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != service.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != service.Description {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotService)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	service := iam.Service{
		Name: fp.Name,
	}

	if fp.Description != nil {
		service.Description = *fp.Description
	}
	if fp.ApplicationID != nil {
		service.ApplicationID = *fp.ApplicationID
	}
	if fp.Scopes != nil {
		service.Scopes = fp.Scopes
	}
	if fp.DefaultScopes != nil {
		service.DefaultScopes = fp.DefaultScopes
	}
	if fp.AccessTokenLifetime != nil {
		service.AccessTokenLifetime = int(*fp.AccessTokenLifetime)
	}
	// Note: RefreshTokenLifetime and TokenEndpointAuthMethod are not supported in Service struct

	created, _, err := e.client.IAM.Services.CreateService(service)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create service")
	}

	meta.SetExternalName(cr, created.ID)

	// Record the observable identity fields. DIP only returns the private key
	// once, at creation time, so we surface it (and the service id) via the
	// connection secret referenced by writeConnectionSecretToRef.
	cr.Status.AtProvider.ID = &created.ID
	cr.Status.AtProvider.ServiceID = &created.ServiceID
	cr.Status.AtProvider.Description = util.StringPtrOrNil(created.Description)
	if created.OrganizationID != "" {
		cr.Status.AtProvider.OrganizationID = &created.OrganizationID
	}
	if created.ExpiresOn != "" {
		cr.Status.AtProvider.ExpiresOn = &created.ExpiresOn
	}

	return managed.ExternalCreation{ConnectionDetails: connectionDetails(created)}, nil
}

// connectionDetails assembles the credential material returned by DIP at
// service creation. The private key is only ever returned once, here.
func connectionDetails(s *iam.Service) managed.ConnectionDetails {
	conn := managed.ConnectionDetails{"serviceId": []byte(s.ServiceID)}
	if s.PrivateKey != "" {
		conn["privateKey"] = []byte(s.PrivateKey)
	}
	if s.OrganizationID != "" {
		conn["organizationId"] = []byte(s.OrganizationID)
	}
	if s.ExpiresOn != "" {
		conn["expiresOn"] = []byte(s.ExpiresOn)
	}
	return conn
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotService)
	}

	fp := cr.Spec.ForProvider

	service := iam.Service{
		ID:   meta.GetExternalName(cr),
		Name: fp.Name,
	}

	if fp.Description != nil {
		service.Description = *fp.Description
	}
	if fp.ApplicationID != nil {
		service.ApplicationID = *fp.ApplicationID
	}
	if fp.Scopes != nil {
		service.Scopes = fp.Scopes
	}
	if fp.DefaultScopes != nil {
		service.DefaultScopes = fp.DefaultScopes
	}
	if fp.AccessTokenLifetime != nil {
		service.AccessTokenLifetime = int(*fp.AccessTokenLifetime)
	}
	// Note: RefreshTokenLifetime and TokenEndpointAuthMethod are not supported in Service struct

	_, _, err := e.client.IAM.Services.UpdateService(service)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update service")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotService)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.Services.DeleteService(iam.Service{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete service")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
