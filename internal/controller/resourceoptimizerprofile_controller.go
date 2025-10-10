/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	ScaleUpAction = "ScaleUp"
)

// ResourceOptimizerProfileReconciler reconciles a ResourceOptimizerProfile object
type ResourceOptimizerProfileReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PrometheusAPI prometheusv1.API
}

// +kubebuilder:rbac:groups=optimizer.example.com,resources=resourceoptimizerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimizer.example.com,resources=resourceoptimizerprofiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimizer.example.com,resources=resourceoptimizerprofiles/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceOptimizerProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch ResourceOptimizerProfile
	var resourceOptimizerProfile optimizerv1.ResourceOptimizerProfile
	if err := r.Get(ctx, req.NamespacedName, &resourceOptimizerProfile); err != nil {
		logger.Error(err, "unable to fetch ResourceOptimizerProfile")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Query Prometheus for metrics
	logger.Info("Querying Prometheus for metrics...")
	query, err := buildPromQL(&resourceOptimizerProfile)
	if err != nil {
		logger.Error(err, "error building PromQL query")
		return ctrl.Result{}, err
	}
	result, err := executePromQL(ctx, r.PrometheusAPI, query)
	if err != nil {
		logger.Error(err, "error querying Prometheus")
		return ctrl.Result{}, err
	}
	logger.Info("Prometheus query result", "result", result)

	// 3. Compare against thresholds
	logger.Info("Comparing metrics against thresholds...")
	var value float64
	switch result.Type() {
	case model.ValVector:
		vector := result.(model.Vector)
		if len(vector) > 0 {
			value = float64(vector[0].Value)
		}
	default:
		logger.Info("Prometheus query did not return a vector")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	cpuThresholds := resourceOptimizerProfile.Spec.CPUThresholds
	var action string

	if value < float64(cpuThresholds.Min) {
		action = "ScaleDown"
	} else if value > float64(cpuThresholds.Max) {
		action = ScaleUpAction
	} else {
		action = "DoNothing"
	}

	logger.Info("Comparison result", "action", action)

	// 4. Execute policy action
	logger.Info("Executing policy action...")
	if err := r.executePolicyAction(ctx, &resourceOptimizerProfile, action); err != nil {
		logger.Error(err, "error executing policy action")
		return ctrl.Result{}, err
	}

	// 5. Update status
	logger.Info("Updating status...")
	resourceOptimizerProfile.Status.ObservedMetrics = map[string]string{"cpu_usage": fmt.Sprintf("%.2f", value)}
	resourceOptimizerProfile.Status.LastAction = action
	if err := r.Status().Update(ctx, &resourceOptimizerProfile); err != nil {
		logger.Error(err, "unable to update ResourceOptimizerProfile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *ResourceOptimizerProfileReconciler) executePolicyAction(ctx context.Context, profile *optimizerv1.ResourceOptimizerProfile, action string) error {
	logger := log.FromContext(ctx)

	if action == "DoNothing" {
		return nil
	}

	labelSelector := labels.Set(profile.Spec.Selector.MatchLabels).AsSelector()

	// List Deployments
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, &client.ListOptions{LabelSelector: labelSelector, Namespace: profile.Namespace}); err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		patch := client.MergeFrom(deployment.DeepCopy())
		var newReplicas int32
		if action == "ScaleUp" {
			newReplicas = *deployment.Spec.Replicas + 1
		} else {
			newReplicas = *deployment.Spec.Replicas - 1
		}

		if newReplicas < 1 {
			newReplicas = 1
		}

		deployment.Spec.Replicas = &newReplicas
		if err := r.Patch(ctx, &deployment, patch); err != nil {
			logger.Error(err, "error patching deployment")
			return err
		}
		logger.Info("Patched deployment", "deployment", deployment.Name, "replicas", newReplicas)
	}

	// List StatefulSets
	var statefulSets appsv1.StatefulSetList
	if err := r.List(ctx, &statefulSets, &client.ListOptions{LabelSelector: labelSelector, Namespace: profile.Namespace}); err != nil {
		return err
	}

	for _, statefulSet := range statefulSets.Items {
		patch := client.MergeFrom(statefulSet.DeepCopy())
		var newReplicas int32
		if action == "ScaleUp" {
			newReplicas = *statefulSet.Spec.Replicas + 1
		} else {
			newReplicas = *statefulSet.Spec.Replicas - 1
		}

		if newReplicas < 1 {
			newReplicas = 1
		}

		statefulSet.Spec.Replicas = &newReplicas
		if err := r.Patch(ctx, &statefulSet, patch); err != nil {
			logger.Error(err, "error patching statefulset")
			return err
		}
		logger.Info("Patched statefulset", "statefulset", statefulSet.Name, "replicas", newReplicas)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceOptimizerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://prometheus-k8s.monitoring.svc.cluster.local:9090"
	}

	promAPI, err := newPrometheusAPI(prometheusURL)
	if err != nil {
		return err
	}

	r.PrometheusAPI = promAPI

	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1.ResourceOptimizerProfile{}).
		Complete(r)
}
