package controller_test

import (
	"context"
	"fmt"
	graphql_api "github.com/PixoVR/pixo-golang-clients/pixo-platform/graphql-api"
	"github.com/go-faker/faker/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	platformv1 "pixovr.com/platform/api/v1"
	"pixovr.com/platform/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	runtime "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var _ = Describe("Pixoserviceaccount", func() {

	var (
		ctx            context.Context
		reconciler     controller.PixoServiceAccountReconciler
		platformClient *graphql_api.MockGraphQLClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		platformClient = &graphql_api.MockGraphQLClient{}
		reconciler = controller.PixoServiceAccountReconciler{
			Client:         k8sClient,
			PlatformClient: platformClient,
		}
	})

	It("can do nothing if the service account is not found", func() {
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      faker.Username(),
				Namespace: Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(platformClient.CalledCreateUser).To(BeFalse())
	})

	Context("when the service account is exists", func() {

		var (
			serviceAccount *platformv1.PixoServiceAccount
			req            ctrl.Request
		)

		BeforeEach(func() {
			serviceAccount = CreateTestServiceAccount(ctx, Namespace)
			req = NewRequest(serviceAccount)
		})

		It("can update the status if user doesnt exist and there is an error creating the user", func() {
			platformClient.GetUserError = true
			platformClient.CreateUserError = true

			result, err := reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(HaveOccurred())
			Expect(platformClient.CalledCreateUser).To(BeTrue())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal("error creating user"))
		})

		It("can create a user if the service account is found", func() {
			platformClient.GetUserError = true

			result, err := reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(platformClient.CalledCreateUser).To(BeTrue())
			Expect(platformClient.CalledCreateAPIKey).To(BeTrue())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal(""))
			Expect(serviceAccount.Status.ID).To(Equal(1))
			Expect(serviceAccount.Status.APIKeyID).NotTo(BeZero())
			ExpectStatusToEqualSpec(serviceAccount)
		})

		It("should create an api key if the service account is found and the user already exists", func() {
			_ = CreateTestSecret(ctx, serviceAccount)

			result, err := reconciler.Reconcile(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(platformClient.CalledCreateUser).To(BeFalse())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal(""))
		})

		It("can update a user if the service account is found", func() {
			_ = CreateTestSecret(ctx, serviceAccount)
			result, err := reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(platformClient.CalledUpdateUser).To(BeTrue())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal(""))
		})

		It("can do nothing if the service account is found but the user update fails", func() {
			platformClient.UpdateUserError = true

			result, err := reconciler.Reconcile(ctx, req)

			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(platformClient.CalledUpdateUser).To(BeTrue())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal("error updating user"))
		})

		It("can delete a user and api key if the service account is deleted", func() {
			platformClient.GetUserError = true
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.Delete(ctx, serviceAccount)).To(Succeed())

			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(platformClient.CalledDeleteAPIKey).To(BeTrue())
			Expect(platformClient.CalledDeleteUser).To(BeTrue())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
			secret := serviceAccount.GenerateAuthSecretSpec()
			Expect(reconciler.Get(ctx, runtime.ObjectKeyFromObject(secret), secret)).To(HaveOccurred())
		})

		It("can do nothing but update the status if the service account is deleted but the api key delete fails", func() {
			platformClient.GetUserError = true
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.Delete(ctx, serviceAccount)).To(Succeed())
			platformClient.DeleteAPIKeyError = true

			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(HaveOccurred())
			Expect(platformClient.CalledDeleteAPIKey).To(BeTrue())
			Expect(platformClient.CalledDeleteUser).To(BeFalse())
			err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
			Expect(err).NotTo(HaveOccurred())
			Expect(serviceAccount.Status.Error).To(Equal("error deleting api key"))
		})

		It("can do nothing but update the status if the service account is deleted but the user delete fails", func() {
			platformClient.GetUserError = true
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(k8sClient.Delete(ctx, serviceAccount)).To(Succeed())
			platformClient.DeleteUserError = true

			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(HaveOccurred())
			Expect(platformClient.CalledDeleteAPIKey).To(BeTrue())
			Expect(platformClient.CalledDeleteUser).To(BeTrue())
			Expect(reconciler.Get(ctx, req.NamespacedName, serviceAccount)).To(Succeed())
			Expect(serviceAccount.Status.Error).To(Equal("error deleting user"))
		})

		It("should add environment variables if the correct annotation is present", func() {
			platformClient.GetUserError = true
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(platformClient.CalledCreateUser).To(BeTrue())
			Expect(platformClient.CalledCreateAPIKey).To(BeTrue())
			deployment := NewTestDeployment(Namespace, "test-deployment", serviceAccount.ObjectMeta.Name)
			Expect(reconciler.Create(ctx, deployment)).Should(Succeed())

			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			result, err = reconciler.Reconcile(ctx, req)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			var updatedDeployment v1.Deployment
			Expect(reconciler.Get(ctx, runtime.ObjectKeyFromObject(deployment), &updatedDeployment)).Should(Succeed())
			ExpectEnvVarsToExist(updatedDeployment, serviceAccount)
		})

		It("should add environment variables even if the user already exists", func() {
			_ = CreateTestSecret(ctx, serviceAccount)
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(platformClient.CalledCreateUser).To(BeFalse())
			deployment := NewTestDeployment(Namespace, "test-deployment-user-exists", serviceAccount.ObjectMeta.Name)
			Expect(reconciler.Create(ctx, deployment)).Should(Succeed())

			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			var updatedDeployment v1.Deployment
			Expect(reconciler.Get(ctx, runtime.ObjectKeyFromObject(deployment), &updatedDeployment)).Should(Succeed())
			ExpectEnvVarsToExist(updatedDeployment, serviceAccount)
		})

		It("should create an api key for a service account that exists but has no api key", func() {
			platformClient.GetUserError = true
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(platformClient.CalledCreateUser).To(BeTrue())
			Expect(platformClient.CalledCreateAPIKey).To(BeTrue())
			secretSpec := serviceAccount.GenerateAuthSecretSpec()
			Expect(k8sClient.Delete(ctx, secretSpec)).To(Succeed())

			platformClient.GetUserError = false
			platformClient.CalledCreateUser = false
			platformClient.CalledCreateAPIKey = false
			result, err = reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(platformClient.CalledCreateAPIKey).To(BeTrue())
			Expect(platformClient.CalledCreateUser).To(BeFalse())
			Expect(reconciler.Get(ctx, req.NamespacedName, serviceAccount)).Should(Succeed())
			Expect(serviceAccount.Status.APIKeyID).NotTo(BeZero())
		})

	})

})

func ExpectEnvVarsToExist(deployment v1.Deployment, serviceAccount *platformv1.PixoServiceAccount) {
	Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
	Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(3))
	Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
		Name:  "PIXO_USERNAME",
		Value: serviceAccount.ObjectMeta.Name,
	}))
	ExpectEnvVarsToContain(deployment, "PIXO_PASSWORD")
	ExpectEnvVarsToContain(deployment, "PIXO_API_KEY")
}

func ExpectEnvVarsToContain(deployment v1.Deployment, key string) {
	Expect(len(deployment.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
	found := false
	for _, envVar := range deployment.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == key {
			found = true
		}
	}
	Expect(found).To(BeTrue())
}

func ExpectStatusToEqualSpec(serviceAccount *platformv1.PixoServiceAccount) {
	Expect(serviceAccount.Status.FirstName).To(Equal(serviceAccount.Spec.FirstName))
	Expect(serviceAccount.Status.LastName).To(Equal(serviceAccount.Spec.LastName))
	Expect(serviceAccount.Status.OrgID).To(Equal(serviceAccount.Spec.OrgID))
	Expect(serviceAccount.Status.Role).To(Equal(serviceAccount.Spec.Role))
	Expect(serviceAccount.Status.APIKeyID).NotTo(BeZero())
}

func NewRequest(serviceAccount *platformv1.PixoServiceAccount) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      serviceAccount.Name,
			Namespace: serviceAccount.Namespace,
		},
	}
}

func CreateTestServiceAccount(ctx context.Context, namespace string) *platformv1.PixoServiceAccount {
	pixoServiceAccount := NewTestServiceAccount(namespace, strings.ToLower(faker.Username()), "admin")
	Expect(pixoServiceAccount).NotTo(BeNil())
	Expect(k8sClient.Create(ctx, pixoServiceAccount)).To(Succeed())
	return pixoServiceAccount
}

func NewTestServiceAccount(namespace, name, role string) *platformv1.PixoServiceAccount {
	return &platformv1.PixoServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: platformv1.PixoServiceAccountSpec{
			FirstName: faker.FirstName(),
			LastName:  faker.LastName(),
			OrgID:     1,
			Role:      role,
		},
	}
}

func NewTestDeployment(namespace, name, serviceAccountName string) *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				controller.AnnotationKey: serviceAccountName,
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  name,
						Image: "nginx",
					}},
				},
			},
		},
	}
}

func CreateTestSecret(ctx context.Context, serviceAccount *platformv1.PixoServiceAccount) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-auth", serviceAccount.ObjectMeta.Name),
			Namespace: serviceAccount.ObjectMeta.Namespace,
		},
		StringData: map[string]string{
			"username": serviceAccount.ObjectMeta.Name,
			"password": "test-password",
		},
		Type: corev1.SecretTypeOpaque,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	return secret
}
