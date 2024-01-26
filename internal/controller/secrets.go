package controller

import (
	"context"
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
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to create api key", nil, err)
	}

	serviceAccount.Status.APIKeyID = apiKey.ID
	if err = r.HandleStatusUpdate(ctx, serviceAccount, "created api key", user, nil); err != nil {
		return err
	}

	secret := serviceAccount.GenerateAuthSecretSpec()
	secret.StringData = map[string]string{
		"username": user.Username,
		"password": password,
		"api-key":  apiKey.Key,
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, secret); err != nil {
				return r.HandleStatusUpdate(ctx, serviceAccount, "failed to create auth secret", user, err)
			}
		}

		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to get auth secret", user, err)
	} else {
		if err = r.Update(ctx, secret); err != nil {
			return r.HandleStatusUpdate(ctx, serviceAccount, "failed to update auth secret", user, err)
		}
	}

	serviceAccount.Status.APIKeyID = apiKey.ID
	return r.HandleStatusUpdate(ctx, serviceAccount, "created auth secret", user, nil)
}

func (r *PixoServiceAccountReconciler) authSecretExists(ctx context.Context, serviceAccount *v1.PixoServiceAccount) bool {
	key := client.ObjectKeyFromObject(serviceAccount.GenerateAuthSecretSpec())
	if err := r.Get(ctx, key, serviceAccount.GenerateAuthSecretSpec()); err != nil {
		return false
	}

	return true
}
