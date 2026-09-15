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
	"context"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/philips-software/go-dip-api/iam"
)

// retryableStatusCodes are HTTP statuses HSDP IAM is known to return
// transiently for group/role assignment calls. 422 in particular: HSDP can
// briefly reject $assign-role/$remove-role with Unprocessable Entity right
// after creating the Group or Role, before the assignment becomes available
// on its side - the terraform-provider-hsdp Group resource works around the
// same behavior with its own outer retry loop on top of the SDK's.
var retryableStatusCodes = map[int]bool{
	http.StatusUnprocessableEntity: true,
	http.StatusTooManyRequests:     true,
	http.StatusInternalServerError: true,
	http.StatusBadGateway:          true,
	http.StatusServiceUnavailable:  true,
	http.StatusGatewayTimeout:      true,
}

// retryTransient retries op with exponential backoff when it fails with one
// of retryableStatusCodes. The go-dip-api SDK already retries some of these
// internally, but not long enough for HSDP's eventual-consistency window
// after a Group or Role was just created - this adds an outer layer on top,
// mirroring terraform-provider-hsdp's Group resource.
func retryTransient(ctx context.Context, op func() (*iam.Response, error)) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.MaxElapsedTime = 20 * time.Second

	return backoff.Retry(func() error {
		resp, err := op()
		if err == nil {
			return nil
		}
		if resp != nil && retryableStatusCodes[resp.StatusCode()] {
			return err
		}
		return backoff.Permanent(err)
	}, backoff.WithContext(backoff.WithMaxRetries(b, 6), ctx))
}
