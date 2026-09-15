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

import (
	"testing"

	iamv1 "github.com/loafoe/provider-hsdp/apis/iam/v1"
)

func strPtr(s string) *string { return &s }

func TestDiffSharingPolicies(t *testing.T) {
	cases := map[string]struct {
		desired       []iamv1.RoleSharingPolicyParameters
		current       []iamv1.RoleSharingPolicyStatus
		wantApplyOrgs []string
		wantRemovOrgs []string
	}{
		"NoChange": {
			desired: []iamv1.RoleSharingPolicyParameters{
				{TargetOrganizationID: strPtr("org-a"), SharingPolicy: "ALL_USERS", Purpose: strPtr("p")},
			},
			current: []iamv1.RoleSharingPolicyStatus{
				{TargetOrganizationID: "org-a", SharingPolicy: "ALL_USERS", Purpose: "p"},
			},
		},
		"NewPolicy": {
			desired: []iamv1.RoleSharingPolicyParameters{
				{TargetOrganizationID: strPtr("org-a"), SharingPolicy: "ALL_USERS"},
			},
			wantApplyOrgs: []string{"org-a"},
		},
		"ChangedPolicyMode": {
			desired: []iamv1.RoleSharingPolicyParameters{
				{TargetOrganizationID: strPtr("org-a"), SharingPolicy: "SPECIFIC_USERS"},
			},
			current: []iamv1.RoleSharingPolicyStatus{
				{TargetOrganizationID: "org-a", SharingPolicy: "ALL_USERS"},
			},
			wantApplyOrgs: []string{"org-a"},
		},
		"RemovedPolicy": {
			current: []iamv1.RoleSharingPolicyStatus{
				{TargetOrganizationID: "org-a", SharingPolicy: "ALL_USERS"},
			},
			wantRemovOrgs: []string{"org-a"},
		},
		"MixedAddAndRemove": {
			desired: []iamv1.RoleSharingPolicyParameters{
				{TargetOrganizationID: strPtr("org-b"), SharingPolicy: "ALL_USERS"},
			},
			current: []iamv1.RoleSharingPolicyStatus{
				{TargetOrganizationID: "org-a", SharingPolicy: "ALL_USERS"},
			},
			wantApplyOrgs: []string{"org-b"},
			wantRemovOrgs: []string{"org-a"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			toApply, toRemove := diffSharingPolicies(tc.desired, tc.current)

			if len(toApply) != len(tc.wantApplyOrgs) {
				t.Fatalf("toApply = %+v, want orgs %v", toApply, tc.wantApplyOrgs)
			}
			for i, a := range toApply {
				if a.TargetOrganizationID == nil || *a.TargetOrganizationID != tc.wantApplyOrgs[i] {
					t.Errorf("toApply[%d] org = %v, want %v", i, a.TargetOrganizationID, tc.wantApplyOrgs[i])
				}
			}

			if len(toRemove) != len(tc.wantRemovOrgs) {
				t.Fatalf("toRemove = %+v, want orgs %v", toRemove, tc.wantRemovOrgs)
			}
			for i, r := range toRemove {
				if r.TargetOrganizationID != tc.wantRemovOrgs[i] {
					t.Errorf("toRemove[%d] org = %v, want %v", i, r.TargetOrganizationID, tc.wantRemovOrgs[i])
				}
			}

			wantUpToDate := len(tc.wantApplyOrgs) == 0 && len(tc.wantRemovOrgs) == 0
			if got := sharingPoliciesUpToDate(tc.desired, tc.current); got != wantUpToDate {
				t.Errorf("sharingPoliciesUpToDate() = %v, want %v", got, wantUpToDate)
			}
		})
	}
}
