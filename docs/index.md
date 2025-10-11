# Welcome to K20s

**K20s** is a smart Kubernetes controller that provides sophisticated, policy-driven workload autoscaling and rightsizing.

Unlike the native Horizontal Pod Autoscaler (HPA), K20s leverages long-term, historical data from Prometheus to make more intelligent and stable scaling decisions. It introduces the `ResourceOptimizerProfile` Custom Resource, allowing you to declaratively define optimization policies for your applications.

## Key Features

- **Policy-Driven Optimization**: Choose between `Scale`, `Resize`, and `Recommend` policies to control how your workloads are optimized.
- **Historical Data Analysis**: Integrates directly with Prometheus to analyze resource utilization over time, avoiding jerky scaling decisions based on momentary spikes.
- **Configurable Thresholds**: Define your ideal CPU utilization "Goldilocks Zone" to keep your applications performant and cost-effective.
- **Built-in Safety**: Features like cooldown periods and min/max boundaries prevent oscillatory scaling and ensure stability.
- **Simple Observability**: A built-in status dashboard and custom Prometheus metrics give you a clear view of what the controller is doing.

Get started by learning how to deploy the controller and use it to optimize your workloads.