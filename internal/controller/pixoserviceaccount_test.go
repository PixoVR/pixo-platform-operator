package controller_test

import (
	"context"
	graphql_api "github.com/PixoVR/pixo-golang-clients/pixo-platform/graphql-api"
	"github.com/go-faker/faker/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
				Namespace: namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
	})

	It("can create a user if the service account is found", func() {
		pixoServiceAccount := NewTestServiceAccount(strings.ToLower(faker.FirstName()), "admin")
		Expect(k8sClient.Create(ctx, pixoServiceAccount)).To(Succeed())
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      pixoServiceAccount.Name,
				Namespace: pixoServiceAccount.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		//Expect(mockPlatformClient.CalledCreateUser).To(BeTrue())
		//
		//mockPlatformClient.GetUserError = true
		//
		//result, err = reconciler.Reconcile(ctx, req)
		//Expect(result).To(Equal(ctrl.Result{}))
		//Expect(err).NotTo(HaveOccurred())
		//Expect(mockPlatformClient.CalledCreateUser).To(BeFalse())
	})

	It("can update a user if the service account is found", func() {
		pixoServiceAccount := NewTestServiceAccount(strings.ToLower(faker.FirstName()), "admin")
		Expect(k8sClient.Create(ctx, pixoServiceAccount)).To(Succeed())
		pixoServiceAccount.Spec.FirstName = faker.FirstName()
		pixoServiceAccount.Spec.LastName = faker.LastName()
		pixoServiceAccount.Spec.Role = "user"
		pixoServiceAccount.Spec.OrgID = 2
		Expect(k8sClient.Update(ctx, pixoServiceAccount)).To(Succeed())
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      pixoServiceAccount.Name,
				Namespace: pixoServiceAccount.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)

		Expect(result).To(Equal(ctrl.Result{}))
		Expect(err).NotTo(HaveOccurred())
		//Expect(mockPlatformClient.CalledUpdateUser).To(BeTrue())
	})

})

func NewTestServiceAccount(name, role string) *platformv1.PixoServiceAccount {
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
