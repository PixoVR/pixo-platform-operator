package controller

import (
	platformv1 "pixovr.com/platform/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *PixoServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.PixoServiceAccount{}).
		Complete(r)
}
