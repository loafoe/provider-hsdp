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

package device

import (
	"context"
	stderrors "errors"
	"time"

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
	errNotDevice    = "managed resource is not a Device"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errNewClient    = "cannot create DIP client"
	errGetPassword  = "cannot get password from secret"
)

// Setup adds a controller that reconciles Device managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.DeviceGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.DeviceGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.Device{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.Device)
	if !ok {
		return nil, errors.New(errNotDevice)
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

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*iamv1.Device)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDevice)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	device, resp, err := e.client.IAM.Devices.GetDeviceByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		// GetDeviceByID searches rather than fetching by path, so a deleted
		// device comes back as a successful response with zero results
		// (ErrNotFound), not a 404 - handle that as not-found too.
		if stderrors.Is(err, iam.ErrNotFound) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get device")
	}
	if device == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &device.ID
	cr.Status.AtProvider.IsActive = &device.IsActive
	if device.RegistrationDate != nil {
		cr.Status.AtProvider.RegistrationDate = util.StringPtrOrNil(device.RegistrationDate.Format(time.RFC3339))
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, device),
	}, nil
}

func (e *external) isUpToDate(cr *iamv1.Device, device *iam.Device) bool {
	fp := cr.Spec.ForProvider

	if fp.Type != device.Type {
		return false
	}
	if fp.Text != nil && *fp.Text != device.Text {
		return false
	}
	if fp.ForTest != nil && *fp.ForTest != device.ForTest {
		return false
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Device)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDevice)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	password, err := e.getPassword(ctx, fp.PasswordSecretRef, e.namespace)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetPassword)
	}

	device := iam.Device{
		LoginID: fp.LoginID,
		DeviceExtID: iam.DeviceIdentifier{
			System: fp.DeviceExtIDSystem,
			Value:  fp.DeviceExtIDValue,
			Type: iam.CodeableConcept{
				Code: fp.DeviceExtIDTypeCode,
			},
		},
		Password:          password,
		Type:              fp.Type,
		OrganizationID:    fp.OrganizationID,
		GlobalReferenceID: fp.GlobalReferenceID,
		ApplicationID:     fp.ApplicationID,
	}

	if fp.DeviceExtIDTypeText != nil {
		device.DeviceExtID.Type.Text = *fp.DeviceExtIDTypeText
	}
	if fp.Text != nil {
		device.Text = *fp.Text
	}
	if fp.ForTest != nil {
		device.ForTest = *fp.ForTest
	}

	created, _, err := e.client.IAM.Devices.CreateDevice(device)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create device")
	}

	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.Device)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDevice)
	}

	fp := cr.Spec.ForProvider

	device := iam.Device{
		ID:   meta.GetExternalName(cr),
		Type: fp.Type,
	}

	if fp.Text != nil {
		device.Text = *fp.Text
	}
	if fp.ForTest != nil {
		device.ForTest = *fp.ForTest
	}

	_, _, err := e.client.IAM.Devices.UpdateDevice(device)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update device")
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.Device)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDevice)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.Devices.DeleteDevice(iam.Device{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete device")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
