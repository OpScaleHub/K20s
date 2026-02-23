# Welcome to K20s Documentation 🚀

K20s is a Kubebuilder-based Kubernetes Controller designed for intelligent, history-aware workload autoscaling. It leverages Prometheus PromQL integrations to make deterministic rightsizing and scaling decisions for your workloads.

---

### 🌟 Project Highlights

- **Smart Analysis:** Instead of relying on instantaneous metric spikes like traditional HPA/VPA, K20s executes PromQL time-series queries to evaluate real sustained load.
- **Dual Mode Action:** 
    - `Scale` horizontally modifies Deployment/StatefulSet replicas based on CPU threshold bounds.
    - `Resize` vertically adjusts CPU resource `requests` while protecting against zero-limits (`1m` minimum flooring).
- **Environment Agnostic:** Fully validated against edge Kubernetes distributions like **k3s**, integrating seamlessly with the `kube-prometheus-stack`.
- **Fault Tolerance:** Built with robust cooldown mechanisms (`.spec.cooldownPeriod`) and nil-pointer resilience to keep your cluster operations stable.

### 📚 Documentation Links
Find your way around the internal documentation:

*   **[API Reference](api-reference.md)**: Deep dive into the `ResourceOptimizerProfile` Custom Resource Definition (CRD) specifications, parameters, and status tracking.
*   **[Makefile Commands](make-help.md)**: Learn about the available `make` commands for building, deploying, and locally testing the controller.

<br>

*Built with ♥ using Go and Kubebuilder.*
