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

package group

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
	errNotGroup     = "managed resource is not a Group"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create DIP client"
)

// Setup adds a controller that reconciles Group managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.GroupGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.GroupGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.Group{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.Group)
	if !ok {
		return nil, errors.New(errNotGroup)
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
	cr, ok := mg.(*iamv1.Group)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGroup)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	group, resp, err := e.client.IAM.Groups.GetGroupByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get group")
	}
	if group == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &group.ID
	cr.Status.AtProvider.Description = util.StringPtrOrNil(group.Description)

	assignedRoleIDs, err := e.assignedRoleIDs(*group)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get roles assigned to group")
	}
	cr.Status.AtProvider.AssignedRoleIDs = assignedRoleIDs

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, group),
	}, nil
}

// assignedRoleIDs returns the GUIDs of the Roles currently assigned to group,
// as observed from DIP.
func (e *external) assignedRoleIDs(group iam.Group) ([]string, error) {
	roles, _, err := e.client.IAM.Groups.GetRoles(group)
	if err != nil {
		return nil, err
	}
	if roles == nil {
		return nil, nil
	}
	ids := make([]string, 0, len(*roles))
	for _, role := range *roles {
		ids = append(ids, role.ID)
	}
	return ids, nil
}

func (e *external) isUpToDate(cr *iamv1.Group, group *iam.Group) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != group.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != group.Description {
		return false
	}
	if !sameIDs(fp.RoleIDs, cr.Status.AtProvider.AssignedRoleIDs) {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Group)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGroup)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	group := iam.Group{
		Name: fp.Name,
	}

	if fp.Description != nil {
		group.Description = *fp.Description
	}
	if fp.ManagingOrganizationID != nil {
		group.ManagingOrganization = *fp.ManagingOrganizationID
	}

	created, _, err := e.client.IAM.Groups.CreateGroup(group)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create group")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.Group)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotGroup)
	}

	fp := cr.Spec.ForProvider

	group := iam.Group{
		ID:   meta.GetExternalName(cr),
		Name: fp.Name,
	}

	if fp.Description != nil {
		group.Description = *fp.Description
	}

	_, _, err := e.client.IAM.Groups.UpdateGroup(group)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update group")
	}

	toAdd, toRemove := diffIDs(fp.RoleIDs, cr.Status.AtProvider.AssignedRoleIDs)
	for _, roleID := range toAdd {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := e.client.IAM.Groups.AssignRole(ctx, group, iam.Role{ID: roleID})
			return resp, err
		}); err != nil {
			return managed.ExternalUpdate{}, errors.Wrapf(err, "cannot assign role %s to group", roleID)
		}
	}
	for _, roleID := range toRemove {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := e.client.IAM.Groups.RemoveRole(ctx, group, iam.Role{ID: roleID})
			return resp, err
		}); err != nil {
			return managed.ExternalUpdate{}, errors.Wrapf(err, "cannot remove role %s from group", roleID)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.Group)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotGroup)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.Groups.DeleteGroup(iam.Group{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete group")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
