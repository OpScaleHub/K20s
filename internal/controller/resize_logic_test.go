package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// mockPrometheusAPI allows us to simulate responses from Prometheus.
type mockPrometheusAPI struct {
	result model.Value
	err    error
}

func (m *mockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.result, nil, nil
}

var _ = Describe("Resize Logic", func() {
	const (
		testNamespace = "default"
		appName       = "test-app"
	)

	var (
		reconciler *ResourceOptimizerProfileReconciler
		profile    *optimizerv1.ResourceOptimizerProfile
		deployment *appsv1.Deployment
	)

	BeforeEach(func() {
		// Create a deployment to be targeted by the optimizer
		initialCPU := resource.MustParse("500m")
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: testNamespace,
				Labels:    map[string]string{"app": appName},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": appName}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": appName}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "main",
							Image: "nginx",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceCPU: initialCPU},
							},
						}},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), deployment)).To(Succeed())

		// Create the optimizer profile
		profile = &optimizerv1.ResourceOptimizerProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "test-profile", Namespace: testNamespace},
			Spec: optimizerv1.ResourceOptimizerProfileSpec{
				Selector:           metav1.LabelSelector{MatchLabels: map[string]string{"app": appName}},
				OptimizationPolicy: "Resize",
				CPUThresholds:      optimizerv1.ThresholdSpec{Min: 30, Max: 70},
			},
		}
		Expect(k8sClient.Create(context.Background(), profile)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), deployment)).To(Succeed())
		Expect(k8sClient.Delete(context.Background(), profile)).To(Succeed())
	})

	Context("When CPU usage is above the max threshold", func() {
		It("should resize the CPU request up", func() {
			// Simulate Prometheus returning 90% usage
			mockAPI := &mockPrometheusAPI{
				result: model.Vector{{Value: 90}},
			}
			reconciler = &ResourceOptimizerProfileReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), PrometheusAPI: mockAPI}

			_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: profile.Name, Namespace: profile.Namespace}})
			Expect(err).NotTo(HaveOccurred())

			// Check that the deployment was patched
			updatedDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: appName, Namespace: testNamespace}, updatedDeployment)).To(Succeed())

			newRequest := updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
			// Expected: (90 / 50) * 500m * 1.25 = 1125m
			Expect(newRequest.String()).To(Equal("1125m"))
		})
	})

	Context("When resizing and a maxCPU limit is set", func() {
		It("should clamp the new CPU request to the maxCPU limit", func() {
			// Set a maxCPU limit on the profile
			profile.Spec.MaxCPU = resource.NewMilliQuantity(1000, resource.DecimalSI) // 1000m
			Expect(k8sClient.Update(context.Background(), profile)).To(Succeed())

			// Simulate 90% usage, which would normally calculate to 1125m
			mockAPI := &mockPrometheusAPI{result: model.Vector{{Value: 90}}}
			reconciler = &ResourceOptimizerProfileReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), PrometheusAPI: mockAPI}

			_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: profile.Name, Namespace: profile.Namespace}})
			Expect(err).NotTo(HaveOccurred())

			updatedDeployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: appName, Namespace: testNamespace}, updatedDeployment)).To(Succeed())

			newRequest := updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
			Expect(newRequest.String()).To(Equal("1")) // 1000m is represented as "1"
		})
	})
})
