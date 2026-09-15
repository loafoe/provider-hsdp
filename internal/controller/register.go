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

package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/loafoe/provider-hsdp/internal/controller/application"
	"github.com/loafoe/provider-hsdp/internal/controller/client"
	"github.com/loafoe/provider-hsdp/internal/controller/config"
	"github.com/loafoe/provider-hsdp/internal/controller/device"
	"github.com/loafoe/provider-hsdp/internal/controller/emailtemplate"
	"github.com/loafoe/provider-hsdp/internal/controller/group"
	mdmapplication "github.com/loafoe/provider-hsdp/internal/controller/mdm/application"
	mdmauthenticationmethod "github.com/loafoe/provider-hsdp/internal/controller/mdm/authenticationmethod"
	mdmdevicegroup "github.com/loafoe/provider-hsdp/internal/controller/mdm/devicegroup"
	mdmdevicetype "github.com/loafoe/provider-hsdp/internal/controller/mdm/devicetype"
	mdmproposition "github.com/loafoe/provider-hsdp/internal/controller/mdm/proposition"
	mdmstandardservice "github.com/loafoe/provider-hsdp/internal/controller/mdm/standardservice"
	"github.com/loafoe/provider-hsdp/internal/controller/organization"
	"github.com/loafoe/provider-hsdp/internal/controller/passwordpolicy"
	"github.com/loafoe/provider-hsdp/internal/controller/proposition"
	provisioningorgconfiguration "github.com/loafoe/provider-hsdp/internal/controller/provisioning/orgconfiguration"
	"github.com/loafoe/provider-hsdp/internal/controller/role"
	"github.com/loafoe/provider-hsdp/internal/controller/service"
	"github.com/loafoe/provider-hsdp/internal/controller/user"
)

// SetupGated creates all DIP controllers with safe-start support and adds them to
// the supplied manager.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		config.Setup,
		// IAM resources
		organization.Setup,
		group.Setup,
		role.Setup,
		proposition.Setup,
		application.Setup,
		client.Setup,
		service.Setup,
		device.Setup,
		user.Setup,
		emailtemplate.Setup,
		passwordpolicy.Setup,
		// MDM resources
		mdmproposition.Setup,
		mdmapplication.Setup,
		mdmstandardservice.Setup,
		mdmdevicegroup.Setup,
		mdmdevicetype.Setup,
		mdmauthenticationmethod.Setup,
		// Provisioning resources
		provisioningorgconfiguration.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
