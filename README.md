# K20s: Smart Kubernetes Workload Autoscaler 🚀

[![pages-build-deployment](https://github.com/OpScaleHub/K20s/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/OpScaleHub/K20s/actions/workflows/pages/pages-build-deployment)

K20s is a Kubebuilder-based Kubernetes Controller designed to perform intelligent workload autoscaling and rightsizing. Unlike native Kubernetes HPA/VPA that react to instantaneous metrics, K20s queries **long-term historical data via Prometheus** (executed through PromQL) to orchestrate state changes, preventing sudden spikes from triggering oscillatory scaling behavior.

---

## 🏗️ Architecture

K20s acts upon the declarative Custom Resource Definition (CRD) `ResourceOptimizerProfile`. 

### Key Capabilities
- **Historical Analysis:** Queries Prometheus deployments directly to calculate time-series averages.
- **Horizontal Scaling (`Scale`):** Adjusts Deployment and StatefulSet `.spec.replicas` when workloads leave the configured "Goldilocks Zone".
- **Vertical Rightsizing (`Resize`):** Modifies pod container CPU resource `requests` based on percentage utilizations, intelligently computing optimal bounds (protecting against zero-rounding errors with a `1m` minimum limit).
- **Safety Measures:** Includes defined `.spec.cooldownPeriod` to prevent rapid consecutive actions, and extensive nil-pointer safeguards for unconfigured targets.

### Supported Environments
- Optimized and tested heavily on lightweight Kubernetes distributions like **k3s**.
- Integrates seamlessly out-of-the-box with `prometheus-community/kube-prometheus-stack`.

---

## 📋 The `ResourceOptimizerProfile` CRD

The operator monitors instances of `ResourceOptimizerProfiles` targetting your workloads.

| Field | Description | Purpose |
| :--- | :--- | :--- |
| **`.spec.selector`** | Standard Kubernetes label selector. | Identifies specific `Deployments` or `StatefulSets` (e.g. `app: my-app`). |
| **`.spec.cpuThresholds`** | `min` and `max` utilization targets. | Keeps average Prometheus CPU requests bounded (e.g., 30/75). |
| **`.spec.optimizationPolicy`**| `Scale`, `Resize`, or `Recommend`. | Decides if it horizontally scales pods or vertically adjusts container requests. |
| **`.spec.cooldownPeriod`** | Go duration string (e.g. `5m`). | Prevents oscillation loops immediately following actions. |
| **`.status.observedMetrics`**| Fetched PromQL output. | Observability into decision-making logic. |
| **`.status.lastAction`** | Timestamp tracking. | Tracks the previous action executed. |

---

## 🛠️ Technology Stack
- **Language:** Go (Golang)
- **Framework:** Kubebuilder / controller-runtime
- **Interfacing:** `k8s.io/client-go` for resource patching, `prometheus/client_golang` for API evaluations.

## 🚀 Getting Started

### 1. Prerequisites
Ensure you have a monitoring stack actively scraping `container_cpu_usage_seconds_total`. 
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace
```

### 2. Deploy the Controller
```bash
# Make sure PROMETHEUS_URL environment variable points to your service
export PROMETHEUS_URL="http://prometheus-operated.monitoring.svc:9090"
make install
make run
```

### 3. Apply a Profile
```yaml
apiVersion: optimizer.k20s.opscale.ir/v1
kind: ResourceOptimizerProfile
metadata:
  name: sample-profile
spec:
  selector:
    matchLabels:
      app: target-workload
  cpuThresholds:
    min: 20
    max: 80
  optimizationPolicy: Scale
  cooldownPeriod: 2m
```
Apply using `kubectl apply -f sample-profile.yaml`.

---

### Project Status
 Development is **complete** for the core engine. Bugs related to Replica nil-pointers and decimal-rounding `0m` CPU limits have been patched in the latest iteration.
