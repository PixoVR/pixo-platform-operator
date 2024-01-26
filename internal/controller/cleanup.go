package controller

import (
	"context"
	v1 "pixovr.com/platform/api/v1"
)

func (r *PixoServiceAccountReconciler) cleanup(ctx context.Context, serviceAccount *v1.PixoServiceAccount) error {
	if serviceAccount.Status.APIKeyID != 0 {
		if err := r.PlatformClient.DeleteAPIKey(ctx, serviceAccount.Status.APIKeyID); err != nil {
			return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete api key", nil, err)
		}
	}

	if err := r.PlatformClient.DeleteUser(ctx, serviceAccount.Status.ID); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete user", nil, err)
	}

	if err := r.Delete(ctx, serviceAccount.GenerateAuthSecretSpec()); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to delete auth secret", nil, err)
	}

	return r.HandleStatusUpdate(ctx, serviceAccount, "cleanup complete", nil, nil)
}
