package controller

import (
	"context"
	"github.com/PixoVR/pixo-golang-clients/pixo-platform/platform"
	v1 "pixovr.com/platform/api/v1"
)

func (r *PixoServiceAccountReconciler) createUser(ctx context.Context, serviceAccount *v1.PixoServiceAccount) (*platform.User, error) {
	input := serviceAccount.GenerateUserSpec()
	password := input.Password

	if err := r.PlatformClient.CreateUser(ctx, input); err != nil {
		return nil, r.HandleStatusUpdate(ctx, serviceAccount, "failed to create pixo user account", 0, nil, err)
	}

	input.Password = password
	return input, nil
}
