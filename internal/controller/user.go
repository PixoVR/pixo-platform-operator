package controller

import (
	"context"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	v1 "pixovr.com/platform/api/v1"
)

func (r *PixoServiceAccountReconciler) createUser(ctx context.Context, serviceAccount *v1.PixoServiceAccount) (*platform.User, error) {
	input := serviceAccount.GenerateUserSpec()

	user, err := r.PlatformClient.CreateUser(ctx, *input)
	if err != nil {
		return nil, r.HandleStatusUpdate(ctx, serviceAccount, "failed to create pixo user account", 0, user, err)
	}

	return user, nil
}
