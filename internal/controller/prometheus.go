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
	// This query calculates the average CPU usage over the last hour and divides it by the requested CPU resources.
	// The result is the CPU utilization as a percentage of the request.
	query := fmt.Sprintf(`
		# Calculate the average CPU usage as a percentage of the requested CPU over the last hour.
		(
			sum(avg_over_time(kube_pod_container_resource_usage{resource="cpu", namespace="%s", pod=~".*%s.*"}[1h])) by (pod)
			and
			sum(kube_pod_container_resource_requests{resource="cpu", namespace="%s", pod=~".*%s.*"}) by (pod)
		)
		/
		sum(kube_pod_container_resource_requests{resource="cpu", namespace="%s", pod=~".*%s.*"}) by (pod) * 100
		`,
		profile.Namespace, profile.Spec.Selector.MatchLabels["app"],
		profile.Namespace, profile.Spec.Selector.MatchLabels["app"],
		profile.Namespace, profile.Spec.Selector.MatchLabels["app"],
	)

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
