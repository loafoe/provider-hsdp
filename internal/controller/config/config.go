/*
Copyright 2020 The Crossplane Authors.

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

package config

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"

	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	apisv1 "github.com/loafoe/provider-hsdp/apis/v1"
)

// Setup adds controllers that reconcile ProviderConfigs and
// ClusterProviderConfigs by accounting for their current usage.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		setupNamespaced,
		setupCluster,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// setupNamespaced adds a controller that reconciles the namespace-scoped
// ProviderConfig by accounting for its current usage.
func setupNamespaced(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apismv1.ProviderConfigGroupKind)

	of := resource.ProviderConfigKinds{
		Config:    apismv1.ProviderConfigGroupVersionKind,
		Usage:     apismv1.ProviderConfigUsageGroupVersionKind,
		UsageList: apismv1.ProviderConfigUsageListGroupVersionKind,
	}

	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apismv1.ProviderConfig{}).
		Watches(&apismv1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// setupCluster adds a controller that reconciles the cluster-scoped
// ClusterProviderConfig by accounting for its current usage. This is the
// default ProviderConfig kind that namespace-scoped managed resources
// reference, per the Crossplane v2 convention.
func setupCluster(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1.ClusterProviderConfigGroupKind)

	of := resource.ProviderConfigKinds{
		Config:    apisv1.ClusterProviderConfigGroupVersionKind,
		Usage:     apismv1.ProviderConfigUsageGroupVersionKind,
		UsageList: apismv1.ProviderConfigUsageListGroupVersionKind,
	}

	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1.ClusterProviderConfig{}).
		Watches(&apismv1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}
