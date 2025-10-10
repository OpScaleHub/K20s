package controller

import (
	"context"
	"fmt"
	"time"

	optimizerv1 "github.com/OpScaleHub/K20s/api/v1"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	selector, err := metav1.LabelSelectorAsSelector(&profile.Spec.Selector)
	if err != nil {
		return "", err
	}

	query := fmt.Sprintf(`avg_over_time(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s"}[1h])`,
		profile.Namespace, selector.String())

	return query, nil
}

func executePromQL(ctx context.Context, promAPI prometheusv1.API, query string) (model.Value, error) {
	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	return result, nil
}
