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

package util

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apismv1 "github.com/loafoe/provider-hsdp/apis/m/v1"
	apisv1 "github.com/loafoe/provider-hsdp/apis/v1"
)

const (
	errMissingPCRef = "managed resource does not reference a ProviderConfig"
	errUnknownPCRef = "managed resource references a ProviderConfig of unsupported kind"
	errGetPC        = "cannot get ProviderConfig"
	errGetClusterPC = "cannot get ClusterProviderConfig"
)

// ResolveProviderConfig fetches the ProviderConfigSpec referenced by mg,
// resolving either a cluster-scoped ClusterProviderConfig or a
// namespace-scoped ProviderConfig depending on the Kind on the managed
// resource's providerConfigRef. An empty Kind defaults to
// ClusterProviderConfig, matching the Crossplane v2 convention. The returned
// key stably identifies the resolved ProviderConfig (kind+namespace+name),
// suitable for caching an authenticated client per ProviderConfig.
func ResolveProviderConfig(ctx context.Context, kube client.Client, mg resource.ModernManaged) (spec *apisv1.ProviderConfigSpec, key string, err error) {
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return nil, "", errors.New(errMissingPCRef)
	}

	switch ref.Kind {
	case "", apisv1.ClusterProviderConfigKind:
		pc := &apisv1.ClusterProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name}, pc); err != nil {
			return nil, "", errors.Wrap(err, errGetClusterPC)
		}
		return &pc.Spec, fmt.Sprintf("%s//%s", apisv1.ClusterProviderConfigKind, ref.Name), nil
	case apismv1.ProviderConfigKind:
		pc := &apismv1.ProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: mg.GetNamespace()}, pc); err != nil {
			return nil, "", errors.Wrap(err, errGetPC)
		}
		return &pc.Spec, fmt.Sprintf("%s/%s/%s", apismv1.ProviderConfigKind, mg.GetNamespace(), ref.Name), nil
	default:
		return nil, "", errors.Errorf("%s: %q", errUnknownPCRef, ref.Kind)
	}
}
