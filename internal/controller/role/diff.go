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

package role

import iamv1 "github.com/loafoe/provider-hsdp/apis/iam/v1"

// diffSharingPolicies returns the desired sharing policies that are missing
// or out of date in current (toApply) and the current sharing policies for
// target organizations no longer present in desired (toRemove). Sharing
// policies are keyed by target organization: only one sharing policy per
// target organization is supported.
func diffSharingPolicies(desired []iamv1.RoleSharingPolicyParameters, current []iamv1.RoleSharingPolicyStatus) (toApply []iamv1.RoleSharingPolicyParameters, toRemove []iamv1.RoleSharingPolicyStatus) {
	currentByOrg := make(map[string]iamv1.RoleSharingPolicyStatus, len(current))
	for _, c := range current {
		currentByOrg[c.TargetOrganizationID] = c
	}

	desiredOrgs := make(map[string]bool, len(desired))
	for _, d := range desired {
		if d.TargetOrganizationID == nil {
			continue
		}
		desiredOrgs[*d.TargetOrganizationID] = true

		c, ok := currentByOrg[*d.TargetOrganizationID]
		purpose := ""
		if d.Purpose != nil {
			purpose = *d.Purpose
		}
		if !ok || c.SharingPolicy != d.SharingPolicy || c.Purpose != purpose {
			toApply = append(toApply, d)
		}
	}

	for _, c := range current {
		if !desiredOrgs[c.TargetOrganizationID] {
			toRemove = append(toRemove, c)
		}
	}

	return toApply, toRemove
}

// sharingPoliciesUpToDate returns true if desired and current represent the
// same set of sharing policies.
func sharingPoliciesUpToDate(desired []iamv1.RoleSharingPolicyParameters, current []iamv1.RoleSharingPolicyStatus) bool {
	toApply, toRemove := diffSharingPolicies(desired, current)
	return len(toApply) == 0 && len(toRemove) == 0
}
