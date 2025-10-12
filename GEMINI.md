# K20s Controller Development Guide

## Project Overview
This is a Kubernetes controller built with Kubebuilder that performs intelligent workload autoscaling based on Prometheus historical metrics. The controller implements a `ResourceOptimizerProfile` CRD to define optimization policies and executes scaling decisions through PromQL queries.

## Architecture Components

### Core Components
- **ResourceOptimizerProfile CRD**: Declarative optimization policies with selectors, thresholds, and policies
- **Controller Reconciler**: Main business logic executing Prometheus queries and scaling decisions
- **Prometheus Integration**: PromQL client for historical metrics analysis
- **Workload Actuator**: Client-go integration for patching Deployments/StatefulSets

### Key Data Flow
1. ResourceOptimizerProfile spec defines target workloads and thresholds
2. Controller queries Prometheus for historical CPU/memory utilization
3. Reconciler compares metrics against thresholds and executes policy actions
4. Status updates track observed metrics and last actions

## Development Workflow

### Initial Setup
```bash
# Initialize Kubebuilder project (example command)
kubebuilder init --domain k20s.opscale.ir --repo github.com/OpScaleHub/K20s
kubebuilder create api --group optimizer --version v1 --kind ResourceOptimizerProfile
```

### Key Implementation Steps
1. **CRD Design** (`api/v1/resourceoptimizerprofile_types.go`)
   - Define Spec with selector, thresholds, optimizationPolicy
   - Define Status with observedMetrics, lastAction, recommendations
   - Add proper validation markers and kubebuilder annotations

2. **Prometheus Client Integration**
   - Use `github.com/prometheus/client_golang/api/prometheus/v1`
   - Implement PromQL query builder for workload-specific metrics
   - Handle time-series data aggregation (avg_over_time patterns)

3. **Controller Logic** (`internal/controller/resourceoptimizerprofile_controller.go`)
   - Implement Reconcile loop with Prometheus queries
   - Add workload discovery via label selectors
   - Implement scaling logic with client-go patches

### Critical Kubebuilder Patterns

#### CRD Field Definitions
```go
type ResourceOptimizerProfileSpec struct {
    // +kubebuilder:validation:Required
    Selector metav1.LabelSelector `json:"selector"`

    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=100
    CPUThresholds ThresholdSpec `json:"cpuThresholds"`

    // +kubebuilder:validation:Enum=Scale;Resize;Recommend
    OptimizationPolicy string `json:"optimizationPolicy"`
}
```

#### Controller Reconciliation
```go
func (r *ResourceOptimizerProfilereconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch ResourceOptimizerProfile
    // 2. Query Prometheus for metrics
    // 3. Compare against thresholds
    // 4. Execute policy action
    // 5. Update status
    return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}
```

### Prometheus Integration Patterns

#### PromQL Query Construction
```go
// Template: avg_over_time(cpu_usage_rate{namespace="{{.Namespace}}", selector="{{.Selector}}"} [1h])
query := fmt.Sprintf(`avg_over_time(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s"} [1h])`,
    namespace, selectorRegex)
```

#### Client-Go Workload Patching
```go
// Scale deployment replicas
patch := client.MergeFrom(deployment.DeepCopy())
deployment.Spec.Replicas = &newReplicas
return r.Patch(ctx, deployment, patch)
```

### Essential Commands

#### Development Cycle
```bash
# Generate manifests and code
make manifests generate

# Run controller locally
make run

# Build and deploy
make docker-build docker-push IMG=controller:latest
make deploy IMG=controller:latest

# Test with sample CR
kubectl apply -f config/samples/
```

#### Debugging
```bash
# View CRD status
kubectl get resourceoptimizerprofiles -o yaml

# Check controller logs (the namespace will be k20s-system)
kubectl logs -n k20s-system deployment/k20s-controller-manager

# Validate CRD installation
kubectl get crd resourceoptimizerprofiles.optimizer.k20s.opscale.ir
```

### Project-Specific Conventions

#### Error Handling Pattern
- Use controller-runtime's `ctrl.Result` for requeue logic
- Log errors with structured logging (`log.Error()`)
- Return errors that require immediate retry, use `RequeueAfter` for polling

#### Metrics Integration
- Expose custom controller metrics via `controller-runtime/pkg/metrics`
- Follow Prometheus naming conventions: `resourceoptimizer_*`
- Track reconciliation duration, errors, and scaling actions

#### Status Management
- Always update `.status.observedMetrics` with latest Prometheus data
- Record `.status.lastAction` to prevent oscillation
- Use `conditions` for detailed status reporting

### Dependencies
```go
// Key imports for implementation
sigs.k8s.io/controller-runtime
k8s.io/client-go
github.com/prometheus/client_golang/api
k8s.io/apimachinery/pkg/labels
```

### Testing Strategy
- Unit tests for PromQL query generation
- Integration tests with fake Prometheus server
- E2E tests with real cluster and test deployments
- Use `envtest` for controller testing with real etcd