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
