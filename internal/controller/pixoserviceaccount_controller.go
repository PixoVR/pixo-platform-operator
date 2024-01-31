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
	graphql "github.com/PixoVR/pixo-golang-clients/pixo-platform/graphql-api"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	platformv1 "pixovr.com/platform/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	finalizerName = "serviceaccount.platform.pixovr.com"
)

const (
	AnnotationKey = "platform.pixovr.com/service-account-name"
)

// PixoServiceAccountReconciler reconciles a PixoServiceAccount object
type PixoServiceAccountReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	PlatformClient graphql.PlatformClient
}

//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=platform.pixovr.com,resources=pixoserviceaccounts/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PixoServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	serviceAccount := &platformv1.PixoServiceAccount{}
	if err := r.Get(ctx, req.NamespacedName, serviceAccount); err != nil {
		if errors.IsNotFound(err) {
			serviceAccount.Log("service account not found", nil)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if serviceAccount.GetDeletionTimestamp() != nil {

		if err := r.cleanup(ctx, serviceAccount); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.removeFinalizer(ctx, serviceAccount); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, r.HandleStatusUpdate(ctx, serviceAccount, "deleted user and api key", 0, nil, nil)
	}

	if err := r.addFinalizer(ctx, serviceAccount); err != nil {
		return ctrl.Result{}, err
	}

	var msg string
	var user *platform.User
	var err error
	var password string

	if user, err = r.PlatformClient.GetUserByUsername(ctx, req.Name); err == nil {
		if err = r.HandleUpdate(ctx, serviceAccount, user); err != nil {
			return ctrl.Result{}, err
		}

		secret, err := r.getSecret(ctx, serviceAccount)
		if err == nil {
			password = string(secret.Data["password"])
		}

	} else {
		if user, err = r.createUser(ctx, serviceAccount); err != nil {
			return ctrl.Result{}, r.HandleStatusUpdate(ctx, serviceAccount, "failed to create pixo user account", 0, user, err)
		}
		password = user.Password
		msg = "successfully created user"
	}

	if exists := r.authSecretExists(ctx, serviceAccount); !exists {
		if err = r.createAPIKey(ctx, serviceAccount, user, password); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err = r.addEnvVarsToDeployments(ctx, serviceAccount); err != nil {
		return ctrl.Result{}, r.HandleStatusUpdate(ctx, serviceAccount, "failed to list deployments", 0, user, err)
	}

	return ctrl.Result{}, r.HandleStatusUpdate(ctx, serviceAccount, msg, 0, user, err)
}

func (r *PixoServiceAccountReconciler) HandleUpdate(ctx context.Context, pixoServiceAccount *platformv1.PixoServiceAccount, user *platform.User) error {
	var shouldUpdate bool

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
			return r.HandleStatusUpdate(ctx, pixoServiceAccount, "failed to update user", 0, user, err)
		}
	}

	return r.HandleStatusUpdate(ctx, pixoServiceAccount, "updated user", 0, user, nil)
}
