package controller_test

import (
	"context"
	graphql_api "github.com/PixoVR/pixo-golang-clients/pixo-platform/graphql-api"
	"github.com/go-faker/faker/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
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
				Namespace: namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
	})

	It("can update the status if user doesnt exist and there is an error creating the user", func() {
		pixoServiceAccount := CreateTestServiceAccount(ctx)
		req := NewRequest(pixoServiceAccount)
		mockPlatformClient.GetUserError = true
		mockPlatformClient.CreateUserError = true

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal("error creating user"))
	})

	It("can create a user if the service account is found", func() {
		pixoServiceAccount := CreateTestServiceAccount(ctx)
		req := NewRequest(pixoServiceAccount)
		mockPlatformClient.GetUserError = true

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal(""))
		Expect(pixoServiceAccount.Status.ID).To(Equal(1))
		Expect(pixoServiceAccount.Status.Username).To(Equal(pixoServiceAccount.Name))
		Expect(pixoServiceAccount.Status.FirstName).To(Equal(pixoServiceAccount.Spec.FirstName))
		Expect(pixoServiceAccount.Status.LastName).To(Equal(pixoServiceAccount.Spec.LastName))
		Expect(pixoServiceAccount.Status.Role).To(Equal(pixoServiceAccount.Spec.Role))
		Expect(pixoServiceAccount.Status.OrgID).To(Equal(pixoServiceAccount.Spec.OrgID))
	})

	It("can do nothing if the service account is found but the user already exists", func() {
		pixoServiceAccount := CreateTestServiceAccount(ctx)
		req := NewRequest(pixoServiceAccount)

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal(""))
	})

	It("can update a user if the service account is found", func() {
		serviceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler)
		req = NewRequest(serviceAccount)

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledUpdateUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, serviceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(serviceAccount.Status.Error).To(Equal(""))
	})

	It("can do nothing if the service account is found but the user update fails", func() {
		pixoServiceAccount := CreateTestServiceAccount(ctx)
		req := NewRequest(pixoServiceAccount)
		mockPlatformClient.UpdateUserError = true

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledUpdateUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).NotTo(HaveOccurred())
		Expect(pixoServiceAccount.Status.Error).To(Equal("error updating user"))
	})

	It("can delete a user if the service account is deleted", func() {
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler)
		Expect(k8sClient.Delete(ctx, pixoServiceAccount)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledDeleteUser).To(BeTrue())
		err = reconciler.Get(ctx, req.NamespacedName, pixoServiceAccount)
		Expect(err).To(HaveOccurred())
	})

	It("can do nothing and update the status if the service account is deleted but the user delete fails", func() {
		pixoServiceAccount, req := CreateAndReconcileTestServiceAccount(ctx, reconciler)
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

})

func CreateAndReconcileTestServiceAccount(ctx context.Context, reconciler controller.PixoServiceAccountReconciler) (*platformv1.PixoServiceAccount, ctrl.Request) {
	pixoServiceAccount := CreateTestServiceAccount(ctx)
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

func CreateTestServiceAccount(ctx context.Context) *platformv1.PixoServiceAccount {
	pixoServiceAccount := NewTestServiceAccount(strings.ToLower(faker.Username()), "admin")
	Expect(k8sClient.Create(ctx, pixoServiceAccount)).To(Succeed())
	return pixoServiceAccount
}

func NewTestServiceAccount(name, role string) *platformv1.PixoServiceAccount {
	return &platformv1.PixoServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: platformv1.PixoServiceAccountSpec{
			FirstName: faker.FirstName(),
			LastName:  faker.LastName(),
			OrgID:     rand.Intn(100),
			Role:      role,
		},
	}
}
