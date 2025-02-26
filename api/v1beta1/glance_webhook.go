/*
Copyright 2022.

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

package v1beta1

import (
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// GlanceDefaults -
type GlanceDefaults struct {
	ContainerImageURL string
}

var glanceDefaults GlanceDefaults

// log is for logging in this package.
var glancelog = logf.Log.WithName("glance-resource")

// SetupGlanceDefaults - initialize Glance spec defaults for use with either internal or external webhooks
func SetupGlanceDefaults(defaults GlanceDefaults) {
	glanceDefaults = defaults
	glancelog.Info("Glance defaults initialized", "defaults", defaults)
}

// SetupWebhookWithManager sets up the webhook with the Manager
func (r *Glance) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-glance-openstack-org-v1beta1-glance,mutating=true,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=mglance.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Glance{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Glance) Default() {
	glancelog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Check if the KeystoneEndpoint matches with a deployed glanceAPI
func (spec *GlanceSpec) isValidKeystoneEP() bool {
	for name := range spec.GlanceAPIs {
		if spec.KeystoneEndpoint == name {
			return true
		}
	}
	return false
}

// Default - set defaults for this Glance spec
func (spec *GlanceSpec) Default() {
	if len(spec.ContainerImage) == 0 {
		spec.ContainerImage = glanceDefaults.ContainerImageURL
	}
	// When no glanceAPI(s) are specified in the top-level CR
	// we build one by default
	// TODO: (fpantano) Set replicas=0 so users are forced to
	// patch the CR and configure a backend.
	if spec.GlanceAPIs == nil || len(spec.GlanceAPIs) == 0 {
		// keystoneEndpoint will match with the only instance
		// deployed by default
		spec.KeystoneEndpoint = "default"
		spec.GlanceAPIs = map[string]GlanceAPITemplate{
			"default": {},
		}
	}
	for key, glanceAPI := range spec.GlanceAPIs {
		// Check the sub-cr ContainerImage parameter
		if glanceAPI.ContainerImage == "" {
			glanceAPI.ContainerImage = glanceDefaults.ContainerImageURL
			spec.GlanceAPIs[key] = glanceAPI
		}
	}
	// In the special case where the GlanceAPI list is composed by a single
	// element, we can omit the "KeystoneEndpoint" spec parameter and default
	// it to that only instance present in the main CR
	if spec.KeystoneEndpoint == "" && len(spec.GlanceAPIs) == 1 {
		for k := range spec.GlanceAPIs {
			spec.KeystoneEndpoint = k
			break
		}
	}
}

//+kubebuilder:webhook:path=/validate-glance-openstack-org-v1beta1-glance,mutating=false,failurePolicy=fail,sideEffects=None,groups=glance.openstack.org,resources=glances,verbs=create;update,versions=v1beta1,name=vglance.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Glance{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateCreate() error {
	glancelog.Info("validate create", "name", r.Name)
	// At creation time, if the CR has an invalid keystoneEndpoint value (that
	// doesn't match with any defined backend), return an error.
	if !r.Spec.isValidKeystoneEP() {
		return errors.New("KeystoneEndpoint is assigned to an invalid glanceAPI instance")
	}

	//TODO:
	// - Check one of the items of the list is the one that should appear in the
	//   keystone catalog, otherwise raise an error because the field is not set!
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateUpdate(old runtime.Object) error {
	glancelog.Info("validate update", "name", r.Name)

	// Type can either be "split" or "single": we do not support changing layout
	// because there's no logic in the operator to scale down the existing statefulset
	// and scale up the new one, hence updating the Spec.GlanceAPI.Type is not supported
	o := old.(*Glance)
	for key, glanceAPI := range r.Spec.GlanceAPIs {
		// When a new entry (new glanceAPI instance) is added in the main CR, it's
		// possible that the old CR used to compare the new map had no entry with
		// the same name. This represent a valid use case and we shouldn't prevent
		// to grow the deployment
		if _, found := o.Spec.GlanceAPIs[key]; !found {
			continue
		}
		// The current glanceAPI exists and the layout is different
		if glanceAPI.Type != o.Spec.GlanceAPIs[key].Type {
			return errors.New("GlanceAPI deployment layout can't be updated")
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Glance) ValidateDelete() error {
	glancelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
