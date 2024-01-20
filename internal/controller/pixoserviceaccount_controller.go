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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	platformv1 "pixovr.com/platform/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	finalizerName = "serviceaccount.platform.pixovr.com"
)

const (
	AnnotationKey = "pixo-platform/service-account-name"
)

// PixoServiceAccountReconciler reconciles a PixoServiceAccount object
type PixoServiceAccountReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	UsersClient graphql.UsersClient
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
	serviceAccount := &platformv1.PixoServiceAccount{}
	if err := r.Get(ctx, req.NamespacedName, serviceAccount); err != nil {
		if errors.IsNotFound(err) {
			serviceAccount.Log("service account not found", nil)
			return ctrl.Result{}, nil
		}

		serviceAccount.Log("failed to get service account", err)
		return ctrl.Result{}, err
	}

	if serviceAccount.GetDeletionTimestamp() != nil {

		if err := r.UsersClient.DeleteUser(ctx, serviceAccount.Status.ID); err != nil {
			msg := "failed to delete user"
			return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, msg, nil, err)
		}

		serviceAccount.SetFinalizers(removeString(serviceAccount.GetFinalizers(), finalizerName))
		if err := r.Update(ctx, serviceAccount); err != nil {
			msg := "failed to remove finalizer"
			return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, msg, nil, err)
		}

		return ctrl.Result{}, nil
	}

	if !containsString(serviceAccount.GetFinalizers(), finalizerName) {
		serviceAccount.SetFinalizers(append(serviceAccount.GetFinalizers(), finalizerName))

		if err := r.Update(ctx, serviceAccount); err != nil {
			msg := "failed to add finalizer"
			return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, msg, nil, err)
		}
	}

	if user, err := r.UsersClient.GetUserByUsername(ctx, req.Name); err == nil {
		serviceAccount.Log("user already exists", nil)
		return ctrl.Result{}, r.HandleUpdate(ctx, serviceAccount, user)
	}

	input := &platform.User{
		Username:  req.Name,
		Password:  faker.Password() + "!",
		FirstName: serviceAccount.Spec.FirstName,
		LastName:  serviceAccount.Spec.LastName,
		Role:      serviceAccount.Spec.Role,
		OrgID:     serviceAccount.Spec.OrgID,
	}
	user, err := r.UsersClient.CreateUser(ctx, *input)
	if err != nil {
		return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, "failed to create pixo user account", user, err)
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

	// see if secret exists
	if err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, secret); err != nil {
				return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, "failed to create auth secret", user, err)
			}
		}
	}

	deployments := appsv1.DeploymentList{}
	if err = r.List(ctx, &deployments, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, "failed to list deployments", user, err)
	}

	for _, deployment := range deployments.Items {
		log.Debug().Msgf("deployment: %s", deployment.Name)
		if serviceAccountName, ok := deployment.Annotations[AnnotationKey]; ok {
			log.Debug().Msgf("deployment: %s has annotation %s", deployment.Name, serviceAccountName)
			updateDeployment(&deployment, serviceAccountName)

			if err = r.Update(ctx, &deployment); err != nil {
				return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, "failed to update deployment with auth creds", user, err)
			}
		}
	}

	return ctrl.Result{}, r.UpdateStatus(ctx, serviceAccount, "created pixo user account", user, nil)
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
		if user, err := r.UsersClient.UpdateUser(ctx, *user); err != nil {
			return r.UpdateStatus(ctx, pixoServiceAccount, "failed to update user", user, err)
		}
	}

	return r.UpdateStatus(ctx, pixoServiceAccount, "updated user", user, nil)
}

func (r *PixoServiceAccountReconciler) UpdateStatus(ctx context.Context, pixoServiceAccount *platformv1.PixoServiceAccount, msg string, user *platform.User, err error) error {

	pixoServiceAccount.Log(msg, err)

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

	return r.Status().Update(ctx, pixoServiceAccount)
}

// SetupWithManager sets up the controller with the Manager.
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

func updateDeployment(deployment *appsv1.Deployment, serviceAccountName string) {
	log.Debug().Msgf("updating deployment: %s", deployment.Name)

	envVars := []corev1.EnvVar{
		{
			Name:  "PIXO_USERNAME",
			Value: serviceAccountName,
		},
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		container.Env = append(container.Env, envVars...)
		deployment.Spec.Template.Spec.Containers[i] = container
	}
}
