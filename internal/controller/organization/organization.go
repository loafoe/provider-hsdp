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

package organization

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
	errNotOrganization = "managed resource is not an Organization"
	errTrackPCUsage    = "cannot track ProviderConfig usage"
	errGetPC           = "cannot get ProviderConfig"
	errGetCreds        = "cannot get credentials"
	errNewClient       = "cannot create DIP client"
)

// Setup adds a controller that reconciles Organization managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.OrganizationGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.OrganizationGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.Organization{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.Organization)
	if !ok {
		return nil, errors.New(errNotOrganization)
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
	cr, ok := mg.(*iamv1.Organization)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotOrganization)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Check if external-name looks like a UUID (basic check)
	// If not, treat it as "resource doesn't exist yet"
	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	org, resp, err := e.client.IAM.Organizations.GetOrganizationByID(externalName)
	if err != nil {
		// If 400 (invalid ID) or 404 (not found), treat as "doesn't exist"
		if resp != nil && (resp.StatusCode() == 400 || resp.StatusCode() == 404) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get organization")
	}
	if org == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &org.ID
	cr.Status.AtProvider.Active = &org.Active
	cr.Status.AtProvider.Name = util.StringPtrOrNil(org.Name)
	cr.Status.AtProvider.DisplayName = util.StringPtrOrNil(org.DisplayName)
	cr.Status.AtProvider.Description = util.StringPtrOrNil(org.Description)
	cr.Status.AtProvider.Type = util.StringPtrOrNil(org.Type)
	cr.Status.AtProvider.ExternalID = util.StringPtrOrNil(org.ExternalID)
	cr.Status.AtProvider.ParentOrgID = util.StringPtrOrNil(org.Parent.Value)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, org),
	}, nil
}

func (e *external) isUpToDate(cr *iamv1.Organization, org *iam.Organization) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != org.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != org.Description {
		return false
	}
	if fp.DisplayName != nil && *fp.DisplayName != org.DisplayName {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Organization)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotOrganization)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	org := iam.Organization{
		Name: fp.Name,
	}

	if fp.Description != nil {
		org.Description = *fp.Description
	}
	if fp.DisplayName != nil {
		org.DisplayName = *fp.DisplayName
	}
	if fp.ParentOrgID != nil {
		org.Parent.Value = *fp.ParentOrgID
	}
	if fp.Type != nil {
		org.Type = *fp.Type
	}
	if fp.ExternalID != nil {
		org.ExternalID = *fp.ExternalID
	}

	created, _, err := e.client.IAM.Organizations.CreateOrganization(org)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create organization")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.Organization)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotOrganization)
	}

	fp := cr.Spec.ForProvider

	org := iam.Organization{
		ID:   meta.GetExternalName(cr),
		Name: fp.Name,
	}

	if fp.Description != nil {
		org.Description = *fp.Description
	}
	if fp.DisplayName != nil {
		org.DisplayName = *fp.DisplayName
	}

	_, _, err := e.client.IAM.Organizations.UpdateOrganization(org)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update organization")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.Organization)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotOrganization)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.Organizations.DeleteOrganization(iam.Organization{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete organization")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
