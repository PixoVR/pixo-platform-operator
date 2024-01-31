package controller

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "pixovr.com/platform/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PixoServiceAccountReconciler) addEnvVarsToDeployments(ctx context.Context, serviceAccount *v1.PixoServiceAccount) error {
	deployments := appsv1.DeploymentList{}
	if err := r.List(ctx, &deployments, client.InNamespace(serviceAccount.Namespace)); err != nil {
		return r.HandleStatusUpdate(ctx, serviceAccount, "failed to list deployments", 0, nil, err)
	}

	for _, deployment := range deployments.Items {
		if serviceAccountName, ok := deployment.Annotations[AnnotationKey]; ok && serviceAccountName == serviceAccount.Name {
			addOrUpdateEnvVars(&deployment, serviceAccount)

			if err := r.Update(ctx, &deployment); err != nil {
				return r.HandleStatusUpdate(ctx, serviceAccount, "failed to update deployment with auth creds", 0, nil, err)
			}
		}
	}

	return nil
}

func addOrUpdateEnvVars(deployment *appsv1.Deployment, serviceAccount *v1.PixoServiceAccount) {
	if serviceAccount == nil {
		return
	}

	envVars := []corev1.EnvVar{
		{
			Name:  "PIXO_USERNAME",
			Value: serviceAccount.Name,
		},
		{
			Name: "PIXO_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: serviceAccount.AuthSecretName(),
					},
					Key: "password",
				},
			},
		},
		{
			Name: "PIXO_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: serviceAccount.AuthSecretName(),
					},
					Key: "api-key",
				},
			},
		},
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		for _, envVar := range envVars {
			exists := false
			for j, existingEnvVar := range container.Env {
				if existingEnvVar.Name == envVar.Name {
					exists = true
					container.Env[j] = envVar
				}
			}
			if !exists {
				container.Env = append(container.Env, envVar)
			}
		}
		deployment.Spec.Template.Spec.Containers[i] = container
	}
}
