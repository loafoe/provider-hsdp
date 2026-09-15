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
	"slices"
	"testing"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

func TestDiffIDs(t *testing.T) {
	cases := map[string]struct {
		desired   []string
		current   []string
		wantAdd   []string
		wantRemov []string
	}{
		"NoChange": {
			desired: []string{"a", "b"},
			current: []string{"a", "b"},
		},
		"NoChangeDifferentOrder": {
			desired: []string{"b", "a"},
			current: []string{"a", "b"},
		},
		"AddOnly": {
			desired: []string{"a", "b"},
			current: []string{"a"},
			wantAdd: []string{"b"},
		},
		"RemoveOnly": {
			desired:   []string{"a"},
			current:   []string{"a", "b"},
			wantRemov: []string{"b"},
		},
		"AddAndRemove": {
			desired:   []string{"a", "c"},
			current:   []string{"a", "b"},
			wantAdd:   []string{"c"},
			wantRemov: []string{"b"},
		},
		"EmptyDesired": {
			desired:   nil,
			current:   []string{"a", "b"},
			wantRemov: []string{"a", "b"},
		},
		"EmptyCurrent": {
			desired: []string{"a", "b"},
			current: nil,
			wantAdd: []string{"a", "b"},
		},
		"BothEmpty": {},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotAdd, gotRemov := diffIDs(tc.desired, tc.current)
			slices.Sort(gotAdd)
			slices.Sort(gotRemov)
			wantAdd := append([]string{}, tc.wantAdd...)
			wantRemov := append([]string{}, tc.wantRemov...)
			slices.Sort(wantAdd)
			slices.Sort(wantRemov)

			if !slices.Equal(gotAdd, wantAdd) {
				t.Errorf("diffIDs() toAdd = %v, want %v", gotAdd, wantAdd)
			}
			if !slices.Equal(gotRemov, wantRemov) {
				t.Errorf("diffIDs() toRemove = %v, want %v", gotRemov, wantRemov)
			}

			wantSame := len(wantAdd) == 0 && len(wantRemov) == 0
			if got := sameIDs(tc.desired, tc.current); got != wantSame {
				t.Errorf("sameIDs() = %v, want %v", got, wantSame)
			}
		})
	}
}

func TestDesiredIDs(t *testing.T) {
	cases := map[string]struct {
		ids      []string
		refs     []xpv1.NamespacedReference
		selector *xpv1.NamespacedSelector
		want     []string
	}{
		"RefsPresentIDsResolved": {
			ids:  []string{"a"},
			refs: []xpv1.NamespacedReference{{Name: "x"}},
			want: []string{"a"},
		},
		"NoRefsNoSelectorStaleIDs": {
			// This is the bug: refs was emptied, but the resolver leaves the
			// cached IDs in place. desiredIDs must treat this as "nothing
			// desired", not "still desired", so a group's last member/role
			// can actually be removed.
			ids:  []string{"a"},
			refs: nil,
			want: nil,
		},
		"NoRefsButSelectorSet": {
			ids:      []string{"a"},
			refs:     nil,
			selector: &xpv1.NamespacedSelector{},
			want:     []string{"a"},
		},
		"NoRefsNoIDsEitherWay": {
			ids:  nil,
			refs: nil,
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := desiredIDs(tc.ids, tc.refs, tc.selector)
			if !slices.Equal(got, tc.want) {
				t.Errorf("desiredIDs() = %v, want %v", got, tc.want)
			}
		})
	}
}
