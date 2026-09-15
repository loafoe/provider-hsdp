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

package proposition

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	mdmv1 "github.com/loafoe/provider-hsdp/apis/mdm/v1"
	"github.com/loafoe/provider-hsdp/internal/clients/dip"
	"github.com/loafoe/provider-hsdp/internal/util"
)

const (
	errNotProposition = "managed resource is not an MDM Proposition"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create DIP client"
)

// Setup adds a controller that reconciles MDM Proposition managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(mdmv1.PropositionGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(mdmv1.PropositionGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&mdmv1.Proposition{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*mdmv1.Proposition)
	if !ok {
		return nil, errors.New(errNotProposition)
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

	return &external{client: dipClient}, nil
}

type external struct {
	client *dip.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*mdmv1.Proposition)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotProposition)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	prop, resp, err := e.client.MDM.Propositions.GetPropositionByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get MDM proposition")
	}
	if prop == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &prop.ID
	cr.Status.AtProvider.Description = util.StringPtrOrNil(prop.Description)
	cr.Status.AtProvider.GlobalReferenceID = util.StringPtrOrNil(prop.GlobalReferenceID)
	cr.Status.AtProvider.Status = util.StringPtrOrNil(prop.Status)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, prop),
	}, nil
}

func (e *external) isUpToDate(cr *mdmv1.Proposition, prop *mdm.Proposition) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != prop.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != prop.Description {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*mdmv1.Proposition)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotProposition)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	prop := mdm.Proposition{
		ResourceType:      "Proposition",
		Name:              fp.Name,
		GlobalReferenceID: fp.GlobalReferenceID,
		OrganizationGuid: mdm.Identifier{
			Value: fp.OrganizationID,
		},
	}

	if fp.Description != nil {
		prop.Description = *fp.Description
	}
	if fp.PropositionGUID != nil {
		prop.PropositionGuid = &mdm.Identifier{Value: *fp.PropositionGUID}
	}
	if fp.Status != nil {
		prop.Status = *fp.Status
	}

	created, _, err := e.client.MDM.Propositions.CreateProposition(prop)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create MDM proposition")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*mdmv1.Proposition)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotProposition)
	}

	fp := cr.Spec.ForProvider

	prop := mdm.Proposition{
		ResourceType:      "Proposition",
		ID:                meta.GetExternalName(cr),
		Name:              fp.Name,
		GlobalReferenceID: fp.GlobalReferenceID,
		OrganizationGuid: mdm.Identifier{
			Value: fp.OrganizationID,
		},
	}

	if fp.Description != nil {
		prop.Description = *fp.Description
	}
	if fp.PropositionGUID != nil {
		prop.PropositionGuid = &mdm.Identifier{Value: *fp.PropositionGUID}
	}
	if fp.Status != nil {
		prop.Status = *fp.Status
	}

	_, _, err := e.client.MDM.Propositions.UpdateProposition(prop)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update MDM proposition")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*mdmv1.Proposition)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotProposition)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	// MDM Propositions don't have a delete endpoint in go-dip-api
	// They are typically soft-deleted by setting status to inactive
	_ = cr

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
