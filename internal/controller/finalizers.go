package controller

import (
	"context"
	v1 "pixovr.com/platform/api/v1"
)

func (r *PixoServiceAccountReconciler) addFinalizer(ctx context.Context, serviceAccount *v1.PixoServiceAccount) error {

	if !containsString(serviceAccount.GetFinalizers(), finalizerName) {
		serviceAccount.SetFinalizers(append(serviceAccount.GetFinalizers(), finalizerName))

		if err := r.Update(ctx, serviceAccount); err != nil {
			return r.HandleStatusUpdate(ctx, serviceAccount, "failed to add finalizer", nil, err)
		}
	}

	return nil
}

func (r *PixoServiceAccountReconciler) removeFinalizer(ctx context.Context, serviceAccount *v1.PixoServiceAccount) error {

	serviceAccount.SetFinalizers(removeString(serviceAccount.GetFinalizers(), finalizerName))
	if err := r.Update(ctx, serviceAccount); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to remove finalizer", nil, err)
	}

	return nil
}
