package controller

import (
	"context"
	"fmt"
	"time"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
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

func buildPromQL(profile *optimizerv1.ResourceOptimizerProfile) (string, error) {
	// Prefer common metrics that are available in most Prometheus setups:
	// - container_cpu_usage_seconds_total (cadvisor/container runtime)
	// - kube_pod_container_resource_requests (kube-state-metrics)
	// The query below computes the CPU usage (cores) as the rate of cpu seconds and
	// divides it by the requested CPU to produce a percentage of requested CPU used.

	// Guard: try to read an "app" label value from the selector; if missing, match all pods in the namespace.
	appLabel := ""
	if profile.Spec.Selector.MatchLabels != nil {
		if v, ok := profile.Spec.Selector.MatchLabels["app"]; ok {
			appLabel = v
		}
	}

	var podMatcher string
	if appLabel == "" {
		podMatcher = ".*"
	} else {
		// match pod names that contain the app label value (common for Deployment-generated pod names)
		podMatcher = fmt.Sprintf(".*%s.*", appLabel)
	}

	// Use a short rate window (5m) to reflect recent usage and compute percentage.
	// Some Prometheus setups expose `container_cpu_usage_seconds_total`, others expose
	// `container_cpu_user_seconds_total`. Add both rates together so the expression works
	// in either environment.
	// The query returns CPU percentage per-pod.
	query := fmt.Sprintf(`
		(
		  sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s"}[5m]))
		  +
		  sum by (pod) (rate(container_cpu_user_seconds_total{namespace="%s", pod=~"%s"}[5m]))
		)
		/
		sum by (pod) (kube_pod_container_resource_requests{namespace="%s", pod=~"%s", resource="cpu"})
		* 100
	`, profile.Namespace, podMatcher, profile.Namespace, podMatcher, profile.Namespace, podMatcher)

	return query, nil
}

func executePromQL(ctx context.Context, promAPI PrometheusClient, query string) (model.Value, error) {
	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	return result, nil
}
