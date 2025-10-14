package controller

import (
	"context"
	"fmt"
	"time"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func newPrometheusAPI(prometheusURL string) (prometheusv1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		return nil, err
	}
	return prometheusv1.NewAPI(client), nil
}

// buildPromQL constructs the Prometheus query to calculate CPU usage percentage.
func buildPromQL(ctx context.Context, k8sClient client.Client, profile *optimizerv1.ResourceOptimizerProfile) (string, error) {
	logger := log.FromContext(ctx)

	// 1. Get the label selector from the profile
	selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.Selector)
	if err != nil {
		return "", fmt.Errorf("invalid label selector: %w", err)
	}

	// 2. Find pods that match the selector
	var podList corev1.PodList
	if err := k8sClient.List(ctx, &podList, &client.ListOptions{Namespace: profile.Namespace, LabelSelector: selector}); err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for selector, skipping query", "selector", selector.String())
		return "", nil // Return an empty query, which will result in 0 usage
	}

	// 3. Construct a regex for pod names to use in the PromQL query
	podNameRegex := ""
	for i, pod := range podList.Items {
		if i > 0 {
			podNameRegex += "|"
		}
		podNameRegex += pod.Name
	}

	// 4. Build the final PromQL query
	// This query calculates the average CPU usage over 5 minutes as a percentage of the CPU request.
	query := fmt.Sprintf(`
		(sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s", container!=""}[5m])) by (pod) / sum(kube_pod_container_resource_requests{resource="cpu", namespace="%s", pod=~"%s", container!=""}) by (pod)) * 100`,
		profile.Namespace, podNameRegex,
		profile.Namespace, podNameRegex,
	)

	return query, nil
}

func executePromQL(ctx context.Context, promAPI PrometheusClient, query string) (model.Value, error) {
	if query == "" {
		return model.Vector{}, nil // Return an empty vector if there's no query
	}
	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		log.FromContext(ctx).Info("Prometheus query returned warnings", "warnings", warnings)
	}
	return result, nil
}

// PrometheusClient defines the interface for a Prometheus API client.
// This simplifies testing by allowing us to mock only the methods we use.
type PrometheusClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error)
}
