package controller_test

import (
	"context"
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
	"strings"
)

var _ = Describe("Pixoserviceaccount", func() {

	var (
		ctx                context.Context
		reconciler         controller.PixoServiceAccountReconciler
		mockPlatformClient *graphql_api.MockGraphQLClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockPlatformClient = &graphql_api.MockGraphQLClient{}
		reconciler = controller.PixoServiceAccountReconciler{
			Client:      k8sClient,
			UsersClient: mockPlatformClient,
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
		Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
	})

	It("can update the status if user doesnt exist and there is an error creating the user", func() {
		mockPlatformClient.GetUserError = true
		mockPlatformClient.CreateUserError = true
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)

		Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		err := reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal("error creating user"))
	})

	It("can create a user if the service account is found", func() {
		mockPlatformClient.GetUserError = true
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)

		Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		err := reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal(""))
		Expect(pixoServiceAccount.Status.ID).To(Equal(1))
		ExpectStatusToEqualSpec(pixoServiceAccount)
	})

	It("can do nothing if the service account is found but the user already exists", func() {
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)

		Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
		err := reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal(""))
	})

	It("can update a user if the service account is found", func() {
		serviceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledUpdateUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(serviceAccount.Status.Error).To(Equal(""))
	})

	It("can do nothing if the service account is found but the user update fails", func() {
		mockPlatformClient.UpdateUserError = true
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)

		Expect(mockPlatformClient.CalledUpdateUser).To(BeTrue())
		err := reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal("error updating user"))
	})

	It("can delete a user if the service account is deleted", func() {
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)
		Expect(k8sClient.Delete(ctx, pixoServiceAccount)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledDeleteUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).To(HaveOccurred())
	})

	It("can do nothing and update the status if the service account is deleted but the user delete fails", func() {
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)
		Expect(k8sClient.Delete(ctx, pixoServiceAccount)).To(Succeed())
		mockPlatformClient.DeleteUserError = true

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledDeleteUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal("error deleting user"))
	})

	It("Should add environment variables if the correct annotation is present", func() {
		mockPlatformClient.GetUserError = true
		serviceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler, Namespace)
		Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		deployment := &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: Namespace,
				Annotations: map[string]string{
					controller.AnnotationKey: serviceAccount.ObjectMeta.Name,
				},
			},
			Spec: v1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test-deployment"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test-deployment"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-deployment",
							Image: "nginx",
						}},
					},
				},
			},
		}
		Expect(reconciler.Create(ctx, deployment)).Should(Succeed())

		result, err := reconciler.Reconcile(ctx, req)
		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())

		deploymentKey := types.NamespacedName{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		}
		var updatedDeployment v1.Deployment
		Expect(reconciler.Get(ctx, deploymentKey, &updatedDeployment)).Should(Succeed())
		Expect(updatedDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name:  "PIXO_USERNAME",
			Value: serviceAccount.ObjectMeta.Name,
		}))
		envVars := updatedDeployment.Spec.Template.Spec.Containers[0].Env
		foundPassword := false
		for _, envVar := range envVars {
			if envVar.Name == "PIXO_PASSWORD" {
				foundPassword = true
			}
		}
		Expect(foundPassword).To(BeTrue())
	})

})

func ExpectStatusToEqualSpec(serviceAccount *platformv1.PixoServiceAccount) {
	Expect(serviceAccount.Status.FirstName).To(Equal(serviceAccount.Spec.FirstName))
	Expect(serviceAccount.Status.LastName).To(Equal(serviceAccount.Spec.LastName))
	Expect(serviceAccount.Status.OrgID).To(Equal(serviceAccount.Spec.OrgID))
	Expect(serviceAccount.Status.Role).To(Equal(serviceAccount.Spec.Role))
}

func CreateAndReconcileTestServiceAccount(ctx context.Context, reconciler controller.PixoServiceAccountReconciler, namespace string) (*platformv1.PixoServiceAccount, ctrl.Request) {
	pixoServiceAccount := CreateTestServiceAccount(ctx, namespace)
	req := NewRequest(pixoServiceAccount)
	result, err := reconciler.Reconcile(ctx, req)
	Expect(result).To(Equal(ctrl.Result{}))
	Expect(err).NotTo(HaveOccurred())
	return pixoServiceAccount, req
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
