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

import xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

// desiredIDs returns ids, unless neither refs nor selector are set, in which
// case it returns nil regardless of ids.
//
// This matters because crossplane-runtime's reference resolver treats an
// already-populated *IDs field as a cache: once resolved, it's left alone
// even if the corresponding *Refs field is later emptied (see
// NamespacedResolutionRequest.IsNoOp/MultiNamespacedResolutionRequest.IsNoOp
// in crossplane-runtime - this is intentional for a single stable identity
// link like "which Organization", but wrong for a membership list like
// roleRefs/userRefs/serviceRefs: removing the last entry should empty the
// group, but the resolver would otherwise leave the stale ID in place
// forever, so Update() would never see anything to remove). Deriving the
// diffing target from *Refs/*Selector presence rather than trusting the
// cached *IDs directly fixes that without needing to change how the
// resolver itself works.
func desiredIDs(ids []string, refs []xpv1.NamespacedReference, selector *xpv1.NamespacedSelector) []string {
	if len(refs) == 0 && selector == nil {
		return nil
	}
	return ids
}

// diffIDs returns the IDs present in desired but not current (toAdd) and the
// IDs present in current but not desired (toRemove).
func diffIDs(desired, current []string) (toAdd, toRemove []string) {
	desiredSet := make(map[string]bool, len(desired))
	for _, id := range desired {
		desiredSet[id] = true
	}
	currentSet := make(map[string]bool, len(current))
	for _, id := range current {
		currentSet[id] = true
	}

	for _, id := range desired {
		if !currentSet[id] {
			toAdd = append(toAdd, id)
		}
	}
	for _, id := range current {
		if !desiredSet[id] {
			toRemove = append(toRemove, id)
		}
	}

	return toAdd, toRemove
}

// sameIDs returns true if desired and current contain the same set of IDs,
// ignoring order and duplicates.
func sameIDs(desired, current []string) bool {
	toAdd, toRemove := diffIDs(desired, current)
	return len(toAdd) == 0 && len(toRemove) == 0
}
