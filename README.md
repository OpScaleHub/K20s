## Project Proposal: K20s 🚀

| Item | Detail |
| :--- | :--- |
| **Project Name** | **K20s** |
| **Primary Goal** | Develop a Kubernetes controller in Golang, using Kubebuilder, to perform "smart" workload autoscaling and rightsizing recommendations based on Prometheus-derived resource utilization metrics. |

[![pages-build-deployment](https://github.com/OpScaleHub/K20s/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/OpScaleHub/K20s/actions/workflows/pages/pages-build-deployment)
---

## 1. Concept and Rationale

The native Kubernetes Horizontal Pod Autoscaler (HPA) and Vertical Pod Autoscaler (VPA) rely primarily on the `metrics-server`'s instantaneous resource usage. The **ResourceOptimizer Controller** will introduce a more sophisticated, policy-driven approach by leveraging the **long-term, historical data** already stored in our Prometheus/Thanos stack.

This project will create a **Custom Resource Definition (CRD)**, the `ResourceOptimizerProfile`, allowing users to declaratively define optimization policies (e.g., "Keep my `env: staging` apps between 30% and 75% CPU utilization") and the desired action.

---

## 2. Key Components

### 2.1. The Custom Resource: `ResourceOptimizerProfile`

This CRD will define the desired monitoring and optimization state for a set of workloads.

| Field | Description | Purpose |
| :--- | :--- | :--- |
| **`.spec.selector`** | Standard Kubernetes label selector. | Targets specific `Deployments` or `StatefulSets`. |
| **`.spec.cpuThresholds`** | `minPercent` and `maxPercent` utilization targets (e.g., 30/75). | Defines the "Goldilocks Zone" for resource usage. |
| **`.spec.optimizationPolicy`**| Policy for automated action (`Scale`, `Resize`, `Recommend`). | Determines what the controller will attempt to change. |
| **`.status.observedMetrics`**| Latest utilization fetched from Prometheus. | Provides observability into the controller's decision-making data. |
| **`.status.lastAction`** | Timestamp and details of the last autoscaling action. | Tracks and prevents rapid, oscillatory changes. |

### 2.2. The Controller Logic (Go Implementation)

The core `Reconcile` loop will involve two critical, challenging steps:

1.  **Prometheus Integration:**
    * Use a dedicated Go client library (e.g., `promql/api/v1`) to construct and execute **PromQL queries** against the Prometheus HTTP API.
    * *Example Query:* `avg_over_time(cpu_usage_rate{job="kubelet", namespace="{{.Namespace}}", selector="{{.Selector}}"} [1h])`
    * This is the most complex part and is essential for Go/Kubebuilder practice.
2.  **Actuation and Reconciliation:**
    * Compare the query result against the `.spec.cpuThresholds`.
    * If a violation occurs and the policy is `Scale`, use the **Client-Go** library to retrieve the owning workload (Deployment/StatefulSet) and **patch the `.spec.replicas`** field accordingly.
    * If the policy is `Recommend`, calculate a new suggested `requests`/`limits` value and populate the `.status.recommendations` field instead of performing an automatic patch.

---

## 3. Technology Stack

* **Language:** Go (Golang)
* **Framework:** Kubebuilder
* **Libraries:** `sigs.k8s.io/controller-runtime`, `k8s.io/client-go`, and a PromQL/Prometheus client library.
* **Environment:** Existing Kubernetes cluster with Prometheus deployed and accessible via a Service.

---

## 4. Deliverables and Success Criteria

| Deliverable | Success Criteria |
| :--- | :--- |
| **`ResourceOptimizerProfile` CRD** | The CRD is correctly installed and validated using `kubectl apply -f`. |
| **Go Controller Implementation** | The controller successfully queries Prometheus and can scale a test deployment up/down based on the profile thresholds. |
| **Prometheus Metrics Exposure** | The controller itself will expose its own performance metrics (via `controller-runtime`) for latency and error rates, which are then scraped by Prometheus. |
| **Clean Codebase** | Adherence to Go idioms and the standard Kubebuilder project layout. |



###
check it out CLI
