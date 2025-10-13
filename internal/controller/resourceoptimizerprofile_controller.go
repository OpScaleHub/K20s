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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	ScaleUpAction    = "ScaleUp"
	ScaleDownAction  = "ScaleDown"
	ResizeUpAction   = "ResizeUp"
	ResizeDownAction = "ResizeDown"
	DoNothing        = "DoNothing"
)

var (
	scaleUpActions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "k20s_scale_up_actions_total",
		Help: "Total number of scale up actions taken",
	})
	scaleDownActions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "k20s_scale_down_actions_total",
		Help: "Total number of scale down actions taken",
	})
	resizeUpActions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "k20s_resize_up_actions_total",
		Help: "Total number of resize up actions taken",
	})
	resizeDownActions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "k20s_resize_down_actions_total",
		Help: "Total number of resize down actions taken",
	})
)

func init() {
	metrics.Registry.MustRegister(scaleUpActions, scaleDownActions, resizeUpActions, resizeDownActions)
}

// PrometheusClient defines the interface for a Prometheus API client.
// This simplifies testing by allowing us to mock only the methods we use.
type PrometheusClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error)
}

// ResourceOptimizerProfileReconciler reconciles a ResourceOptimizerProfile object
type ResourceOptimizerProfileReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PrometheusAPI PrometheusClient
	// PrometheusURL records the URL used to connect to Prometheus (for logging/debugging)
	PrometheusURL string
}

// +kubebuilder:rbac:groups=optimizer.k20s.opscale.ir,resources=resourceoptimizerprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimizer.k20s.opscale.ir,resources=resourceoptimizerprofiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimizer.k20s.opscale.ir,resources=resourceoptimizerprofiles/finalizers,verbs=update
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
	// Log the query and Prometheus endpoint to make DNS/connectivity problems obvious
	logger.Info("Built PromQL query", "query", query, "prometheusURL", r.PrometheusURL)
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
			// Average across all returned pod series to derive a representative value
			var sum float64
			// Log each sample for debugging
			for _, sample := range vector {
				// attempt to extract pod label, fall back to the full metric
				pod := "unknown"
				if m, ok := sample.Metric["pod"]; ok {
					pod = string(m)
				}
				log.FromContext(ctx).Info("Prometheus sample", "pod", pod, "value", float64(sample.Value))
				sum += float64(sample.Value)
			}
			value = sum / float64(len(vector))
			log.FromContext(ctx).Info("Computed CPU percent (average)", "value", value, "seriesCount", len(vector))
		}
	default:
		logger.Info("Prometheus query did not return a vector")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	cpuThresholds := resourceOptimizerProfile.Spec.CPUThresholds
	var action string

	if value < float64(cpuThresholds.Min) {
		if resourceOptimizerProfile.Spec.OptimizationPolicy == "Resize" {
			action = ResizeDownAction
		} else {
			action = ScaleDownAction
		}
	} else if value > float64(cpuThresholds.Max) {
		if resourceOptimizerProfile.Spec.OptimizationPolicy == "Resize" {
			action = ResizeUpAction
		} else {
			action = ScaleUpAction
		}
	} else {
		action = DoNothing
	}

	logger.Info("Comparison result", "action", action)

	// 4. Handle actions based on the optimization policy
	switch resourceOptimizerProfile.Spec.OptimizationPolicy {
	case "Scale":
		// Policy is "Scale", so we proceed with action execution
		cooldownPeriod := 5 * time.Minute // Default cooldown
		if resourceOptimizerProfile.Spec.CooldownPeriod != nil {
			cooldownPeriod = resourceOptimizerProfile.Spec.CooldownPeriod.Duration
		}
		logger.Info("Using cooldown period", "cooldown", cooldownPeriod.String())

		lastAction := resourceOptimizerProfile.Status.LastAction

		if action != DoNothing && lastAction != nil && lastAction.Type != DoNothing {
			if time.Since(lastAction.Timestamp.Time) < cooldownPeriod {
				logger.Info("Action is in cooldown period, skipping execution", "action", action, "lastActionTimestamp", lastAction.Timestamp)
				// Requeue after the cooldown period expires
				requeueAfter := cooldownPeriod - time.Since(lastAction.Timestamp.Time)
				return ctrl.Result{RequeueAfter: requeueAfter}, nil
			}
		}

		logger.Info("Executing policy action...")
		if err := r.executeScaleAction(ctx, &resourceOptimizerProfile, action); err != nil {
			logger.Error(err, "error executing scale action")
			return ctrl.Result{}, err
		}

		switch action {
		case ScaleUpAction:
			scaleUpActions.Inc()
		case ScaleDownAction:
			scaleDownActions.Inc()
		}

		if action != DoNothing {
			resourceOptimizerProfile.Status.LastAction = &optimizerv1.ActionDetail{
				Type:      action,
				Timestamp: metav1.Now(),
				Details:   fmt.Sprintf("CPU usage was %.2f, triggered %s", value, action),
			}
		}
	case "Resize":
		cooldownPeriod := 5 * time.Minute // Default cooldown
		if resourceOptimizerProfile.Spec.CooldownPeriod != nil {
			cooldownPeriod = resourceOptimizerProfile.Spec.CooldownPeriod.Duration
		}
		logger.Info("Using cooldown period for Resize", "cooldown", cooldownPeriod.String())

		lastAction := resourceOptimizerProfile.Status.LastAction
		if action != DoNothing && lastAction != nil && lastAction.Type != DoNothing {
			if time.Since(lastAction.Timestamp.Time) < cooldownPeriod {
				logger.Info("Action is in cooldown period, skipping execution", "action", action, "lastActionTimestamp", lastAction.Timestamp)
				requeueAfter := cooldownPeriod - time.Since(lastAction.Timestamp.Time)
				return ctrl.Result{RequeueAfter: requeueAfter}, nil
			}
		}

		logger.Info("Executing resize action...")
		if err := r.executeResizeAction(ctx, &resourceOptimizerProfile, action, value); err != nil {
			logger.Error(err, "error executing resize action")
			return ctrl.Result{}, err
		}

		switch action {
		case ResizeUpAction:
			resizeUpActions.Inc()
		case ResizeDownAction:
			resizeDownActions.Inc()
		}

		if action != DoNothing {
			resourceOptimizerProfile.Status.LastAction = &optimizerv1.ActionDetail{
				Type:      action,
				Timestamp: metav1.Now(),
				Details:   fmt.Sprintf("CPU usage was %.2f%%, triggered %s", value, action),
			}
		}

	case "Recommend":
		if action != DoNothing {
			recommendation := fmt.Sprintf("CPU usage is %.2f%%. Consider %s.", value, action)
			resourceOptimizerProfile.Status.Recommendations = []string{recommendation}
		} else {
			// For recommend policy, we clear previous recommendations if no action is needed now
			resourceOptimizerProfile.Status.Recommendations = nil
		}
	default:
		logger.Info("OptimizationPolicy is not 'Scale' or 'Recommend', no action will be taken.", "policy", resourceOptimizerProfile.Spec.OptimizationPolicy)
	}

	// 5. Update status for all policies
	logger.Info("Updating status...")
	resourceOptimizerProfile.Status.ObservedMetrics = map[string]string{"cpu_usage": fmt.Sprintf("%.2f", value)}
	if err := r.Status().Update(ctx, &resourceOptimizerProfile); err != nil {
		logger.Error(err, "unable to update ResourceOptimizerProfile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *ResourceOptimizerProfileReconciler) executeScaleAction(ctx context.Context, profile *optimizerv1.ResourceOptimizerProfile, action string) error {
	logger := log.FromContext(ctx)

	if action == DoNothing {
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
		if action == ScaleUpAction {
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
		if action == ScaleUpAction {
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

func (r *ResourceOptimizerProfileReconciler) executeResizeAction(ctx context.Context, profile *optimizerv1.ResourceOptimizerProfile, action string, observedValue float64) error {
	logger := log.FromContext(ctx)

	if action == DoNothing {
		return nil
	}

	labelSelector := labels.Set(profile.Spec.Selector.MatchLabels).AsSelector()

	// --- Handle Deployments ---
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, &client.ListOptions{LabelSelector: labelSelector, Namespace: profile.Namespace}); err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		patch := client.MergeFrom(deployment.DeepCopy())

		// Iterate over containers and update the first one with a CPU request
		for i, container := range deployment.Spec.Template.Spec.Containers {
			if _, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				// Simple resize logic: target usage is the middle of the threshold range
				targetUsagePercent := (float64(profile.Spec.CPUThresholds.Min+profile.Spec.CPUThresholds.Max) / 2)
				// Calculate new request based on observed usage to meet the target percentage
				// newRequest = (currentUsage / targetPercent)
				newCPUValue := (observedValue / targetUsagePercent) * container.Resources.Requests.Cpu().AsApproximateFloat64()

				// Add a 25% buffer for safety
				newCPUValue *= 1.25

				newCPURequest := resource.NewMilliQuantity(int64(newCPUValue*1000), resource.DecimalSI)

				// Enforce min/max boundaries if they are defined in the spec
				if profile.Spec.MinCPU != nil && newCPURequest.Cmp(*profile.Spec.MinCPU) < 0 {
					newCPURequest = profile.Spec.MinCPU
					logger.Info("Clamping CPU request to configured minCPU", "deployment", deployment.Name, "minCPU", profile.Spec.MinCPU.String())
				}
				if profile.Spec.MaxCPU != nil && newCPURequest.Cmp(*profile.Spec.MaxCPU) > 0 {
					newCPURequest = profile.Spec.MaxCPU
					logger.Info("Clamping CPU request to configured maxCPU", "deployment", deployment.Name, "maxCPU", profile.Spec.MaxCPU.String())
				}

				deployment.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = *newCPURequest

				if err := r.Patch(ctx, &deployment, patch); err != nil {
					logger.Error(err, "error patching deployment for resize")
					return err
				}
				logger.Info("Patched deployment for resize", "deployment", deployment.Name, "newCPURequest", newCPURequest.String())
				break // Only patch the first container with CPU requests for now
			}
		}
	}

	// --- Handle StatefulSets (similar logic) ---
	var statefulSets appsv1.StatefulSetList
	if err := r.List(ctx, &statefulSets, &client.ListOptions{LabelSelector: labelSelector, Namespace: profile.Namespace}); err != nil {
		return err
	}

	for _, ss := range statefulSets.Items {
		patch := client.MergeFrom(ss.DeepCopy())

		for i, container := range ss.Spec.Template.Spec.Containers {
			if _, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				targetUsagePercent := (float64(profile.Spec.CPUThresholds.Min+profile.Spec.CPUThresholds.Max) / 2)
				newCPUValue := (observedValue / targetUsagePercent) * container.Resources.Requests.Cpu().AsApproximateFloat64()
				newCPUValue *= 1.25 // Add 25% buffer

				newCPURequest := resource.NewMilliQuantity(int64(newCPUValue*1000), resource.DecimalSI)

				// Enforce min/max boundaries if they are defined in the spec
				if profile.Spec.MinCPU != nil && newCPURequest.Cmp(*profile.Spec.MinCPU) < 0 {
					newCPURequest = profile.Spec.MinCPU
					logger.Info("Clamping CPU request to configured minCPU", "statefulset", ss.Name, "minCPU", profile.Spec.MinCPU.String())
				}
				if profile.Spec.MaxCPU != nil && newCPURequest.Cmp(*profile.Spec.MaxCPU) > 0 {
					newCPURequest = profile.Spec.MaxCPU
					logger.Info("Clamping CPU request to configured maxCPU", "statefulset", ss.Name, "maxCPU", profile.Spec.MaxCPU.String())
				}

				ss.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = *newCPURequest

				if err := r.Patch(ctx, &ss, patch); err != nil {
					logger.Error(err, "error patching statefulset for resize")
					return err
				}
				logger.Info("Patched statefulset for resize", "statefulset", ss.Name, "newCPURequest", newCPURequest.String())
				break // Only patch the first container with CPU requests
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceOptimizerProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://prometheus-operated.monitoring.svc.cluster.local:9090"
	}

	promAPI, err := newPrometheusAPI(prometheusURL)
	if err != nil {
		return err
	}

	r.PrometheusAPI = promAPI
	r.PrometheusURL = prometheusURL

	// Log chosen Prometheus URL on setup so local runs show connectivity target
	ctrl.Log.WithName("setup").Info("Prometheus URL configured", "url", prometheusURL)

	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1.ResourceOptimizerProfile{}).
		Complete(r)
}
