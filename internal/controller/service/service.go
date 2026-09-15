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
	"crypto/x509"
	"encoding/pem"
	stderrors "errors"

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

	return &external{client: dipClient, kube: c.kube, namespace: m.GetNamespace()}, nil
}

type external struct {
	client    *dip.Client
	kube      client.Client
	namespace string
}

// getSecretValue reads key (defaulting to defaultKey if unset) from the
// secret referenced by ref, defaulting the secret's namespace to namespace.
func (e *external) getSecretValue(ctx context.Context, ref xpv1.SecretKeySelector, namespace, defaultKey string) (string, error) {
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
		key = defaultKey
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", errors.Errorf("secret %s/%s does not have key %s", nn.Namespace, nn.Name, key)
	}

	return string(value), nil
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
		// GetServiceByID searches rather than fetching by path, so a deleted
		// service comes back as a successful response with zero results
		// (ErrEmptyResults), not a 404 - handle that as not-found too.
		if stderrors.Is(err, iam.ErrEmptyResults) {
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

// applyServiceFields copies the optional creatable fields of fp onto service.
func applyServiceFields(service *iam.Service, fp *iamv1.ServiceParameters) {
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
	if fp.Validity != nil {
		service.Validity = int(*fp.Validity)
	}
	// Note: RefreshTokenLifetime and TokenEndpointAuthMethod are not supported in Service struct
}

// applySelfManagedCredentials uploads a self-managed private key or
// certificate to the just-created service, if requested by fp.
func (e *external) applySelfManagedCredentials(ctx context.Context, created iam.Service, fp *iamv1.ServiceParameters) error {
	if fp.PrivateKeySecretRef != nil {
		if err := e.setSelfManagedPrivateKey(ctx, created, *fp.PrivateKeySecretRef); err != nil {
			return errors.Wrap(err, "cannot set self-managed private key")
		}
	}
	if fp.SelfManagedCertificateSecretRef != nil {
		if err := e.setSelfManagedCertificate(ctx, created, *fp.SelfManagedCertificateSecretRef); err != nil {
			return errors.Wrap(err, "cannot set self-managed certificate")
		}
	}
	return nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*iamv1.Service)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotService)
	}

	cr.Status.SetConditions(xpv1.Creating())

	fp := cr.Spec.ForProvider

	if fp.PrivateKeySecretRef != nil && fp.SelfManagedCertificateSecretRef != nil {
		return managed.ExternalCreation{}, errors.New("privateKeySecretRef and selfManagedCertificateSecretRef are mutually exclusive")
	}

	service := iam.Service{
		Name: fp.Name,
	}
	applyServiceFields(&service, &fp)

	created, _, err := e.client.IAM.Services.CreateService(service)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create service")
	}

	if err := e.applySelfManagedCredentials(ctx, *created, &fp); err != nil {
		return managed.ExternalCreation{}, err
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

// setSelfManagedPrivateKey reads a PEM RSA private key from the secret
// referenced by ref, generates a self-signed certificate from it, and
// uploads it to DIP in place of the provider-generated key pair.
func (e *external) setSelfManagedPrivateKey(ctx context.Context, service iam.Service, ref xpv1.SecretKeySelector) error {
	pemKey, err := e.getSecretValue(ctx, ref, e.namespace, "privateKey")
	if err != nil {
		return err
	}

	block, _ := pem.Decode([]byte(iam.FixPEM(pemKey)))
	if block == nil {
		return errors.New("cannot decode PEM private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "cannot parse private key")
	}

	_, _, err = e.client.IAM.Services.UpdateServiceCertificate(service, privateKey)
	return err
}

// setSelfManagedCertificate reads a PEM x509 certificate from the secret
// referenced by ref and uploads it to DIP in place of the
// provider-generated certificate.
func (e *external) setSelfManagedCertificate(ctx context.Context, service iam.Service, ref xpv1.SecretKeySelector) error {
	pemCert, err := e.getSecretValue(ctx, ref, e.namespace, "certificate")
	if err != nil {
		return err
	}

	block, _ := pem.Decode([]byte(iam.FixPEM(pemCert)))
	if block == nil {
		return errors.New("cannot decode PEM certificate")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return errors.Wrap(err, "cannot parse certificate")
	}

	_, _, err = e.client.IAM.Services.UpdateServiceCertificateDER(service, block.Bytes)
	return err
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
	applyServiceFields(&service, &fp)

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
