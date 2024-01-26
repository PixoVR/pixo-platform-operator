package controller

import (
	"context"
	v1 "pixovr.com/platform/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

func (r *PixoServiceAccountReconciler) cleanup(ctx context.Context, serviceAccount *v1.PixoServiceAccount) error {
	secret := serviceAccount.GenerateAuthSecretSpec()
	if err := r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to get auth secret", 0, nil, err)
	}

	if serviceAccount.Status.APIKeyID == 0 {
		apiKeyIDValue, ok := secret.Labels["platform.pixovr.com/api-key-id"]
		if !ok {
			return r.HandleStatusUpdate(ctx, serviceAccount, "no api key id found", 0, nil, nil)
		}

		apiKeyID, err := strconv.Atoi(apiKeyIDValue)
		if err != nil {
			return r.HandleStatusUpdate(ctx, serviceAccount, "invalid api key id", 0, nil, err)
		}

		serviceAccount.Status.APIKeyID = apiKeyID
	}

	if err := r.PlatformClient.DeleteAPIKey(ctx, serviceAccount.Status.APIKeyID); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete api key", 0, nil, err)
	}

	if err := r.PlatformClient.DeleteUser(ctx, serviceAccount.Status.ID); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete user", 0, nil, err)
	}

	if err := r.Delete(ctx, serviceAccount.GenerateAuthSecretSpec()); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete auth secret", 0, nil, err)
	}

	return r.HandleStatusUpdate(ctx, serviceAccount, "cleanup complete", 0, nil, nil)
}
