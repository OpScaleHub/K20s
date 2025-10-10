# API Reference

The `ResourceOptimizerProfile` is a Custom Resource Definition (CRD) that allows you to define how K20s should monitor and optimize your workloads.

## ResourceOptimizerProfile Spec

| Field | Type | Description | Required |
| :--- | :--- | :--- | :--- |
| **`selector`** | `metav1.LabelSelector` | A standard Kubernetes label selector to target the `Deployments` or `StatefulSets` you want to optimize. | Yes |
| **`optimizationPolicy`** | `string` | The action the controller should take. Must be one of `Scale`, `Resize`, or `Recommend`. | Yes |
| **`cpuThresholds`** | `object` | The target CPU utilization range. | Yes |
| **`cpuThresholds.min`** | `integer` | The lower bound of the target CPU utilization percentage (1-100). If usage falls below this, a scale-down or resize-down action is triggered. | Yes |
| **`cpuThresholds.max`** | `integer` | The upper bound of the target CPU utilization percentage (1-100). If usage exceeds this, a scale-up or resize-up action is triggered. | Yes |
| **`cooldownPeriod`** | `string` | The duration the controller will wait before taking another action (e.g., `"5m"`, `"1h"`). Defaults to 5 minutes. | No |
| **`minCPU`** | `string` | For the `Resize` policy only. The minimum CPU request that can be set (e.g., `"50m"`). | No |
| **`maxCPU`** | `string` | For the `Resize` policy only. The maximum CPU request that can be set (e.g., `"1"` for 1 core). | No |

## ResourceOptimizerProfile Status

The `.status` field is updated by the controller to provide real-time observability.

| Field | Type | Description |
| :--- | :--- | :--- |
| **`observedMetrics`** | `map[string]string` | The latest metric value fetched from Prometheus (e.g., `cpu_usage: "15.50"`). |
| **`lastAction`** | `object` | Details about the last action taken by the controller, including `type`, `timestamp`, and `details`. |
| **`recommendations`** | `[]string` | A list of human-readable recommendations when using the `Recommend` policy. |
| **`conditions`** | `[]metav1.Condition` | Standard Kubernetes conditions to report the overall health and status of the profile. |