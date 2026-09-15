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

package emailtemplate

import (
	"context"
	"encoding/base64"

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
	errNotEmailTemplate = "managed resource is not an EmailTemplate"
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetPC            = "cannot get ProviderConfig"
	errGetCreds         = "cannot get credentials"
	errNewClient        = "cannot create DIP client"
)

// Setup adds a controller that reconciles EmailTemplate managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(iamv1.EmailTemplateGroupKind)

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

	r := managed.NewReconciler(mgr, resource.ManagedKind(iamv1.EmailTemplateGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&iamv1.EmailTemplate{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*iamv1.EmailTemplate)
	if !ok {
		return nil, errors.New(errNotEmailTemplate)
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
	cr, ok := mg.(*iamv1.EmailTemplate)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotEmailTemplate)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if !util.IsValidUUID(externalName) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	template, resp, err := e.client.IAM.EmailTemplates.GetTemplateByID(externalName)
	if err != nil {
		if resp != nil && util.IsNotFoundOrInvalidID(resp.StatusCode()) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get email template")
	}
	if template == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.ID = &template.ID
	cr.Status.AtProvider.Subject = util.StringPtrOrNil(template.Subject)
	cr.Status.AtProvider.From = util.StringPtrOrNil(template.From)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: e.isUpToDate(cr, template),
	}, nil
}

func (e *external) isUpToDate(cr *iamv1.EmailTemplate, template *iam.EmailTemplate) bool {
	fp := cr.Spec.ForProvider

	if fp.Type != template.Type {
		return false
	}
	if fp.Subject != nil && *fp.Subject != template.Subject {
		return false
	}
	// Message can't be diffed here: DIP's read API never returns it, and
	// Update() is a no-op (DIP doesn't support updating email templates).
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.EmailTemplate)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotEmailTemplate)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	template := iam.EmailTemplate{
		Type: fp.Type,
	}

	if fp.ManagingOrganizationID != nil {
		template.ManagingOrganization = *fp.ManagingOrganizationID
	}
	if fp.Format != nil {
		template.Format = *fp.Format
	}
	if fp.Locale != nil {
		template.Locale = *fp.Locale
	}
	if fp.Subject != nil {
		template.Subject = *fp.Subject
	}
	if fp.From != nil {
		template.From = *fp.From
	}
	if fp.Message != nil {
		// DIP requires the message body to be base64-encoded on the wire.
		template.Message = base64.StdEncoding.EncodeToString([]byte(*fp.Message))
	}
	if fp.Link != nil {
		template.Link = *fp.Link
	}

	created, _, err := e.client.IAM.EmailTemplates.CreateTemplate(template)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create email template")
	}

	meta.SetExternalName(cr, created.ID)
	cr.Status.AtProvider.MessageBase64 = util.StringPtrOrNil(created.Message)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*iamv1.EmailTemplate)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotEmailTemplate)
	}

	// Note: The DIP API doesn't support updating email templates
	// This is a limitation of the IAM API
	_ = cr

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*iamv1.EmailTemplate)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotEmailTemplate)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, _, err := e.client.IAM.EmailTemplates.DeleteTemplate(iam.EmailTemplate{ID: externalName})
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete email template")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error {
	return nil
}
