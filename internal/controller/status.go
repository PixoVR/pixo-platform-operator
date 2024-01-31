package controller

import (
	"context"
	platform "github.com/PixoVR/pixo-golang-clients/pixo-platform/primary-api"
	"k8s.io/client-go/util/retry"
	platformv1 "pixovr.com/platform/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PixoServiceAccountReconciler) HandleStatusUpdate(ctx context.Context, serviceAccount *platformv1.PixoServiceAccount, msg string, apiKeyID int, user *platform.User, err error) error {
	retryFunc := func() error {
		return r.UpdateStatus(ctx, serviceAccount, msg, apiKeyID, user, err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, retryFunc)
}

func (r *PixoServiceAccountReconciler) UpdateStatus(ctx context.Context, serviceAccount *platformv1.PixoServiceAccount, msg string, apiKeyID int, user *platform.User, err error) error {

	serviceAccount.Log(msg, err)

	update := false

	if apiKeyID != 0 && apiKeyID != serviceAccount.Status.APIKeyID {
		update = true
		serviceAccount.Status.APIKeyID = apiKeyID
	}

	hasNewErr := err != nil && serviceAccount.Status.Error != err.Error()
	if hasNewErr {
		update = true
		serviceAccount.Status.Error = err.Error()
	}

	shouldRemoveError := err == nil && serviceAccount.Status.Error != ""
	if shouldRemoveError {
		update = true
		serviceAccount.Status.Error = ""
	}

	if user != nil {
		update = true
		serviceAccount.Status.ID = user.ID
		serviceAccount.Status.Username = user.Username
		serviceAccount.Status.FirstName = user.FirstName
		serviceAccount.Status.LastName = user.LastName
		serviceAccount.Status.OrgID = user.OrgID
		serviceAccount.Status.Role = user.Role
	}

	if update {
		if updateErr := r.Status().Patch(ctx, serviceAccount, client.Merge); updateErr != nil {
			serviceAccount.Log("failed to update status", updateErr)
		}
	}

	return err
}
