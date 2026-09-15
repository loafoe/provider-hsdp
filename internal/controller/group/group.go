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

	if err := e.populateAssignedIDs(cr, externalName, *group); err != nil {
		return managed.ExternalObservation{}, err
	}

	desiredUserIDs, err := e.desiredUserIDs(ctx, cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot resolve desired group members")
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, group, desiredUserIDs),
	}, nil
}

// populateAssignedIDs reads back the roles/users/services currently assigned
// to the group from DIP and stores them on cr.Status.AtProvider, for isUpToDate
// to diff against.
func (e *external) populateAssignedIDs(cr *iamv1.Group, externalName string, group iam.Group) error {
	assignedRoleIDs, err := e.assignedRoleIDs(group)
	if err != nil {
		return errors.Wrap(err, "cannot get roles assigned to group")
	}
	cr.Status.AtProvider.AssignedRoleIDs = assignedRoleIDs

	assignedUserIDs, err := e.assignedMemberIDs(externalName, "User")
	if err != nil {
		return errors.Wrap(err, "cannot get users assigned to group")
	}
	cr.Status.AtProvider.AssignedUserIDs = assignedUserIDs

	assignedServiceIDs, err := e.assignedMemberIDs(externalName, "Service")
	if err != nil {
		return errors.Wrap(err, "cannot get services assigned to group")
	}
	cr.Status.AtProvider.AssignedServiceIDs = assignedServiceIDs

	assignedDeviceIDs, err := e.assignedMemberIDs(externalName, "Device")
	if err != nil {
		return errors.Wrap(err, "cannot get devices assigned to group")
	}
	cr.Status.AtProvider.AssignedDeviceIDs = assignedDeviceIDs

	return nil
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

// assignedMemberIDs returns the GUIDs of the members of the given type
// (e.g. "User", "Service", "Device") currently in the group with the given
// external name, as observed from DIP via the SCIM API.
func (e *external) assignedMemberIDs(externalName, memberType string) ([]string, error) {
	scimGroup, _, err := e.client.IAM.Groups.SCIMGetGroupByID(externalName, &iam.SCIMGetGroupOptions{
		IncludeGroupMembersType: &memberType,
	})
	if err != nil {
		return nil, err
	}
	if scimGroup == nil {
		return nil, nil
	}
	resources := scimGroup.ExtensionGroup.GroupMembers.Resources
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ID)
	}
	return ids, nil
}

// desiredUserIDs returns the full set of User GUIDs that should be members
// of the group: those resolved via userRefs/userSelector into UserIDs, plus
// those resolved live from userLogins.
func (e *external) desiredUserIDs(ctx context.Context, fp iamv1.GroupParameters) ([]string, error) {
	ids := desiredIDs(fp.UserIDs, fp.UserRefs, fp.UserSelector)
	if len(fp.UserLogins) == 0 {
		return ids, nil
	}
	loginIDs, err := e.resolveUserLogins(ctx, fp.UserLogins)
	if err != nil {
		return nil, err
	}
	return append(ids, loginIDs...), nil
}

// resolveUserLogins resolves each of the given DIP login IDs to a User GUID.
// Uses the legacy lookup (a different, older IDM endpoint) rather than
// GetUserIDByLoginID: the latter is scoped to the caller's own organization
// and returns 403 for a login belonging to a different organization, while
// the legacy endpoint resolves logins across organizations.
func (e *external) resolveUserLogins(ctx context.Context, logins []string) ([]string, error) {
	ids := make([]string, 0, len(logins))
	for _, login := range logins {
		var id string
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			var resp *iam.Response
			var err error
			id, resp, err = e.client.IAM.Users.LegacyGetUserIDByLoginID(login)
			return resp, err
		}); err != nil {
			return nil, errors.Wrapf(err, "cannot resolve user login %q", login)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (e *external) isUpToDate(cr *iamv1.Group, group *iam.Group, desiredUserIDs []string) bool {
	fp := cr.Spec.ForProvider

	if fp.Name != group.Name {
		return false
	}
	if fp.Description != nil && *fp.Description != group.Description {
		return false
	}
	if !sameIDs(desiredIDs(fp.RoleIDs, fp.RoleRefs, fp.RoleSelector), cr.Status.AtProvider.AssignedRoleIDs) {
		return false
	}
	if !sameIDs(desiredUserIDs, cr.Status.AtProvider.AssignedUserIDs) {
		return false
	}
	if !sameIDs(desiredIDs(fp.ServiceIDs, fp.ServiceRefs, fp.ServiceSelector), cr.Status.AtProvider.AssignedServiceIDs) {
		return false
	}
	if !sameIDs(desiredIDs(fp.DeviceIDs, fp.DeviceRefs, fp.DeviceSelector), cr.Status.AtProvider.AssignedDeviceIDs) {
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

	toAddRoles, toRemoveRoles := diffIDs(desiredIDs(fp.RoleIDs, fp.RoleRefs, fp.RoleSelector), cr.Status.AtProvider.AssignedRoleIDs)
	if err := e.reconcileRoles(ctx, group, toAddRoles, toRemoveRoles); err != nil {
		return managed.ExternalUpdate{}, err
	}

	desiredUserIDs, err := e.desiredUserIDs(ctx, fp)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot resolve desired group members")
	}
	toAddUsers, toRemoveUsers := diffIDs(desiredUserIDs, cr.Status.AtProvider.AssignedUserIDs)
	if err := e.reconcileMembers(ctx, group, toAddUsers, toRemoveUsers,
		e.client.IAM.Groups.AddMembers, e.client.IAM.Groups.RemoveMembers, "members"); err != nil {
		return managed.ExternalUpdate{}, err
	}

	toAddServices, toRemoveServices := diffIDs(desiredIDs(fp.ServiceIDs, fp.ServiceRefs, fp.ServiceSelector), cr.Status.AtProvider.AssignedServiceIDs)
	if err := e.reconcileMembers(ctx, group, toAddServices, toRemoveServices,
		e.client.IAM.Groups.AddServices, e.client.IAM.Groups.RemoveServices, "services"); err != nil {
		return managed.ExternalUpdate{}, err
	}

	toAddDevices, toRemoveDevices := diffIDs(desiredIDs(fp.DeviceIDs, fp.DeviceRefs, fp.DeviceSelector), cr.Status.AtProvider.AssignedDeviceIDs)
	if err := e.reconcileMembers(ctx, group, toAddDevices, toRemoveDevices,
		e.client.IAM.Groups.AddDevices, e.client.IAM.Groups.RemoveDevices, "devices"); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, nil
}

// reconcileRoles assigns/removes roles on group one at a time, since the
// underlying DIP API only supports a single role per call.
func (e *external) reconcileRoles(ctx context.Context, group iam.Group, toAdd, toRemove []string) error {
	for _, roleID := range toAdd {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := e.client.IAM.Groups.AssignRole(ctx, group, iam.Role{ID: roleID})
			return resp, err
		}); err != nil {
			return errors.Wrapf(err, "cannot assign role %s to group", roleID)
		}
	}
	for _, roleID := range toRemove {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := e.client.IAM.Groups.RemoveRole(ctx, group, iam.Role{ID: roleID})
			return resp, err
		}); err != nil {
			return errors.Wrapf(err, "cannot remove role %s from group", roleID)
		}
	}
	return nil
}

// reconcileMembers adds/removes members (users or services) on group in bulk,
// via the given add/remove SDK calls, retrying transient DIP errors.
func (e *external) reconcileMembers(ctx context.Context, group iam.Group, toAdd, toRemove []string,
	add, remove func(context.Context, iam.Group, ...string) (iam.MemberResponse, *iam.Response, error), kind string) error {
	if len(toAdd) > 0 {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := add(ctx, group, toAdd...)
			return resp, err
		}); err != nil {
			return errors.Wrapf(err, "cannot add %s to group", kind)
		}
	}
	if len(toRemove) > 0 {
		if err := retryTransient(ctx, func() (*iam.Response, error) {
			_, resp, err := remove(ctx, group, toRemove...)
			return resp, err
		}); err != nil {
			return errors.Wrapf(err, "cannot remove %s from group", kind)
		}
	}
	return nil
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

	// HSDP refuses to delete a Group that still has roles or members
	// assigned (DELETE /Group/{id} returns 409 "Edit state conflict").
	// Crossplane's managed reconciler skips Update() while a resource is
	// being deleted, so this is the only chance to empty the group first,
	// using the most recent Observe()'s assigned-ID snapshot.
	group := iam.Group{ID: externalName}
	if err := e.reconcileRoles(ctx, group, nil, cr.Status.AtProvider.AssignedRoleIDs); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot remove roles before deleting group")
	}
	if err := e.reconcileMembers(ctx, group, nil, cr.Status.AtProvider.AssignedUserIDs,
		e.client.IAM.Groups.AddMembers, e.client.IAM.Groups.RemoveMembers, "members"); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot remove members before deleting group")
	}
	if err := e.reconcileMembers(ctx, group, nil, cr.Status.AtProvider.AssignedServiceIDs,
		e.client.IAM.Groups.AddServices, e.client.IAM.Groups.RemoveServices, "services"); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot remove services before deleting group")
	}
	if err := e.reconcileMembers(ctx, group, nil, cr.Status.AtProvider.AssignedDeviceIDs,
		e.client.IAM.Groups.AddDevices, e.client.IAM.Groups.RemoveDevices, "devices"); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot remove devices before deleting group")
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
