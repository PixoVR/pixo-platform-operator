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

package controller

import (
	"context"
	"fmt"
	graphql "github.com/PixoVR/pixo-golang-clients/pixo-platform/graphql-api"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	"github.com/go-faker/faker/v4"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1 "pixovr.com/platform/api/v1"
)

var (
	finalizerName = "serviceaccount.platform.pixovr.com"
)

// PixoServiceAccountReconciler reconciles a PixoServiceAccount object
type PixoServiceAccountReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	PlatformClient *graphql.GraphQLAPIClient
}

//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts/finalizers,verbs=update
//+kubebuilder:rbac:groups=platform.pixovr.com,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *PixoServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pixoServiceAccount := &platformv1.PixoServiceAccount{}
	if err := r.Get(ctx, req.NamespacedName, pixoServiceAccount); err != nil {
		if errors.IsNotFound(err) {
			log.Error().
				Err(err).
				Str("name", pixoServiceAccount.Name).
				Str("namespace", pixoServiceAccount.Namespace).
				Msg("failed to get pixo service account")
			return ctrl.Result{}, nil
		}

		log.Error().
			Err(err).
			Str("name", pixoServiceAccount.Name).
			Str("namespace", pixoServiceAccount.Namespace).
			Msg("failed to get pixo service account")

		return ctrl.Result{}, err
	}

	if pixoServiceAccount.GetDeletionTimestamp() != nil {

		if err := r.PlatformClient.DeleteUser(ctx, pixoServiceAccount.Status.ID); err != nil {
			log.Info().
				Int("id", pixoServiceAccount.Status.ID).
				Msg("failed to delete user")
			if err = r.UpdateStatus(ctx, pixoServiceAccount, nil, err); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		pixoServiceAccount.SetFinalizers(removeString(pixoServiceAccount.GetFinalizers(), finalizerName))
		if err := r.Update(ctx, pixoServiceAccount); err != nil {
			log.Error().
				Err(err).
				Str("name", pixoServiceAccount.Name).
				Str("namespace", pixoServiceAccount.Namespace).
				Msg("failed to update pixo service account")
			if err = r.UpdateStatus(ctx, pixoServiceAccount, nil, err); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if !containsString(pixoServiceAccount.GetFinalizers(), finalizerName) {
		pixoServiceAccount.SetFinalizers(append(pixoServiceAccount.GetFinalizers(), finalizerName))

		if err := r.Update(ctx, pixoServiceAccount); err != nil {
			log.Error().
				Err(err).
				Str("name", pixoServiceAccount.Name).
				Str("namespace", pixoServiceAccount.Namespace).
				Msg("failed to update pixo service account")
			return ctrl.Result{}, r.UpdateStatus(ctx, pixoServiceAccount, nil, err)
		}
	}

	if user, err := r.PlatformClient.GetUserByUsername(ctx, req.Name); err == nil {
		log.Info().
			Str("username", user.Username).
			Msg("user already exists")

		return ctrl.Result{}, r.HandleUpdate(ctx, pixoServiceAccount, user)
	}

	input := &platform.User{
		Username:  req.Name,
		FirstName: pixoServiceAccount.Spec.FirstName,
		LastName:  pixoServiceAccount.Spec.LastName,
		Role:      pixoServiceAccount.Spec.Role,
		Password:  faker.Password() + "!",
		OrgID:     pixoServiceAccount.Spec.OrgID,
	}
	if user, err := r.PlatformClient.CreateUser(ctx, *input); err != nil {
		log.Error().
			Err(err).
			Str("name", pixoServiceAccount.Name).
			Str("role", pixoServiceAccount.Spec.Role).
			Int("orgID", pixoServiceAccount.Spec.OrgID).
			Msg("failed to create pixo user account")
		return ctrl.Result{}, r.UpdateStatus(ctx, pixoServiceAccount, user, err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-auth", req.Name),
			Namespace: req.Namespace,
		},
		StringData: map[string]string{
			"username": req.Name,
			"password": input.Password,
		},
		Type: v1.SecretTypeOpaque,
	}

	if err := r.Client.Create(ctx, secret); err != nil {
		log.Error().
			Err(err).
			Str("secret", secret.Name).
			Msg("failed to create secret")

		return ctrl.Result{}, r.UpdateStatus(ctx, pixoServiceAccount, nil, err)
	}

	return ctrl.Result{}, nil
}

func (r *PixoServiceAccountReconciler) HandleUpdate(ctx context.Context, pixoServiceAccount *platformv1.PixoServiceAccount, user *platform.User) error {
	var shouldUpdate bool

	if pixoServiceAccount.Name != user.Username {
		shouldUpdate = true
		user.Username = pixoServiceAccount.Name
	}

	if pixoServiceAccount.Spec.FirstName != user.FirstName {
		shouldUpdate = true
		user.FirstName = pixoServiceAccount.Spec.FirstName
	}

	if pixoServiceAccount.Spec.LastName != user.LastName {
		shouldUpdate = true
		user.LastName = pixoServiceAccount.Spec.LastName
	}

	if pixoServiceAccount.Spec.Role != user.Role {
		shouldUpdate = true
		user.Role = pixoServiceAccount.Spec.Role
	}

	if pixoServiceAccount.Spec.OrgID != user.OrgID {
		shouldUpdate = true
		user.OrgID = pixoServiceAccount.Spec.OrgID
	}

	if shouldUpdate {
		if user, err := r.PlatformClient.UpdateUser(ctx, *user); err != nil {
			log.Error().
				Err(err).
				Str("username", user.Username).
				Msg("failed to update user")

			return r.UpdateStatus(ctx, pixoServiceAccount, user, err)
		}
	}

	return r.UpdateStatus(ctx, pixoServiceAccount, user, nil)
}

func (r *PixoServiceAccountReconciler) UpdateStatus(ctx context.Context, pixoServiceAccount *platformv1.PixoServiceAccount, user *platform.User, err error) error {

	if err != nil {
		pixoServiceAccount.Status.Error = err.Error()
	} else {
		pixoServiceAccount.Status.Error = ""
	}

	if user != nil {
		pixoServiceAccount.Status.ID = user.ID
		pixoServiceAccount.Status.Username = user.Username
		pixoServiceAccount.Status.FirstName = user.FirstName
		pixoServiceAccount.Status.LastName = user.LastName
		pixoServiceAccount.Status.OrgID = user.OrgID
		pixoServiceAccount.Status.Role = user.Role
		pixoServiceAccount.Status.Error = ""
	}

	if err = r.Status().Update(ctx, pixoServiceAccount); err != nil {
		log.Error().
			Err(err).
			Str("name", pixoServiceAccount.Name).
			Str("namespace", pixoServiceAccount.Namespace).
			Msg("failed to update pixo service account status")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PixoServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.PixoServiceAccount{}).
		Complete(r)
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
