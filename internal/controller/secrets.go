package controller

import (
	"context"
	"fmt"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	v1 "pixovr.com/platform/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PixoServiceAccountReconciler) getSecret(ctx context.Context, serviceAccount *v1.PixoServiceAccount) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: serviceAccount.AuthSecretName(), Namespace: serviceAccount.Namespace}, secret)
	return secret, err
}

func (r *PixoServiceAccountReconciler) createAPIKey(ctx context.Context, serviceAccount *v1.PixoServiceAccount, user *platform.User, password string) error {
	apiKey, err := r.PlatformClient.CreateAPIKey(ctx, platform.APIKey{UserID: serviceAccount.Status.ID})
	if err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to create api key", 0, nil, err)
	}

	if err = r.HandleStatusUpdate(ctx, serviceAccount, "created api key", apiKey.ID, user, nil); err != nil {
		return err
	}

	secret := serviceAccount.GenerateAuthSecretSpec()
	secret.Labels = map[string]string{
		"platform.pixovr.com/service-account-name": serviceAccount.Name,
		"platform.pixovr.com/api-key-id":           fmt.Sprint(apiKey.ID),
		"platform.pixovr.com/user-id":              fmt.Sprint(user.ID),
		"platform.pixovr.com/username":             user.Username,
	}
	secret.StringData = map[string]string{
		"username": user.Username,
		"password": password,
		"api-key":  apiKey.Key,
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, secret); err != nil {
				return r.HandleStatusUpdate(ctx, serviceAccount, "failed to create auth secret", 0, user, err)
			}
		}

		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to get auth secret", 0, user, err)
	} else {
		if err = r.Update(ctx, secret); err != nil {
			return r.HandleStatusUpdate(ctx, serviceAccount, "failed to update auth secret", 0, user, err)
		}
	}

	return r.HandleStatusUpdate(ctx, serviceAccount, "created auth secret", apiKey.ID, user, nil)
}

func (r *PixoServiceAccountReconciler) authSecretExists(ctx context.Context, serviceAccount *v1.PixoServiceAccount) bool {
	key := client.ObjectKeyFromObject(serviceAccount.GenerateAuthSecretSpec())
	if err := r.Get(ctx, key, serviceAccount.GenerateAuthSecretSpec()); err != nil {
		return false
	}

	return true
}
