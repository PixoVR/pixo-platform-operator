/*
Copyright 2024.

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

package v1

import (
	"fmt"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	"github.com/go-faker/faker/v4"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PixoServiceAccountSpec defines the desired state of PixoServiceAccount
type PixoServiceAccountSpec struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	OrgID     int    `json:"orgId,omitempty"`
	Role      string `json:"role,omitempty"`
}

// PixoServiceAccountStatus defines the observed state of PixoServiceAccount
type PixoServiceAccountStatus struct {
	ID        int    `json:"id,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Username  string `json:"username,omitempty"`
	OrgID     int    `json:"orgId,omitempty"`
	Role      string `json:"role,omitempty"`
	APIKeyID  int    `json:"apiKeyId,omitempty"`
	Error     string `json:"error,omitempty"`

	CreatedAt metav1.Time `json:"createdAt,omitempty"`
	UpdatedAt metav1.Time `json:"updatedAt,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PixoServiceAccount is the Schema for the pixoserviceaccounts API
type PixoServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PixoServiceAccountSpec   `json:"spec,omitempty"`
	Status PixoServiceAccountStatus `json:"status,omitempty"`
}

func (s *PixoServiceAccount) Log(msg string, err error) {
	if err != nil {
		log.Error().
			Err(err).
			Str("name", s.Name).
			Str("namespace", s.Namespace).
			Msg(msg)
	}

	log.Info().
		Str("name", s.Name).
		Str("namespace", s.Namespace).
		Msg(msg)
}

func (p *PixoServiceAccount) AuthSecretName() string {
	return fmt.Sprintf("%s-auth", p.Name)
}

func (p *PixoServiceAccount) GenerateUserSpec() *platform.User {
	return &platform.User{
		Username:  p.Name,
		Password:  faker.Password() + "!",
		FirstName: p.Spec.FirstName,
		LastName:  p.Spec.LastName,
		Role:      p.Spec.Role,
		OrgID:     p.Spec.OrgID,
	}
}

func (p *PixoServiceAccount) GenerateAuthSecretSpec() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.AuthSecretName(),
			Namespace: p.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

//+kubebuilder:object:root=true

// PixoServiceAccountList contains a list of PixoServiceAccount
type PixoServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PixoServiceAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&PixoServiceAccount{},
		&PixoServiceAccountList{},
	)
}
