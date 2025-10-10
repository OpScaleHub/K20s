/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/prometheus/common/model"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
)

var _ = Describe("ResourceOptimizerProfile Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		resourceoptimizerprofile := &optimizerv1.ResourceOptimizerProfile{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ResourceOptimizerProfile")
			err := k8sClient.Get(ctx, typeNamespacedName, resourceoptimizerprofile)
			if err != nil && errors.IsNotFound(err) {
				resource := &optimizerv1.ResourceOptimizerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: optimizerv1.ResourceOptimizerProfileSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test-app"},
						},
						CPUThresholds: optimizerv1.ThresholdSpec{
							Min: 30,
							Max: 70,
						},
						OptimizationPolicy: "Recommend",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &optimizerv1.ResourceOptimizerProfile{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ResourceOptimizerProfile")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ResourceOptimizerProfileReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				// Provide a mock API to prevent nil pointer dereference
				PrometheusAPI: &mockPrometheusAPI{
					result: model.Vector{}, // Return an empty vector for this basic test
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
