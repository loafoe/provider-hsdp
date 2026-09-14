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

import "regexp"

// uuidRegex matches standard UUID format
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsValidUUID checks if a string looks like a UUID
func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

// IsNotFoundOrInvalidID returns true if the HTTP status code indicates
// the resource doesn't exist or the ID is invalid
func IsNotFoundOrInvalidID(statusCode int) bool {
	return statusCode == 400 || statusCode == 404
}

// StringPtrOrNil returns a pointer to s, or nil if s is empty. Useful for
// populating optional *string observation fields from a plain string API
// response without surfacing an empty string as "set".
func StringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
