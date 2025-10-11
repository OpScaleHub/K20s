# Getting Started

Deploying the K20s controller to your Kubernetes cluster is straightforward.

## Prerequisites

- A Kubernetes cluster (v1.25+)
- `kubectl` installed
- `make` installed
- A running Prometheus instance that is accessible from within the cluster.

## Deployment

### 1. Clone the Repository

```bash
git clone https://github.com/OpScaleHub/K20s.git
cd K20s
```

### 2. Deploy the Controller

The `make deploy` command uses Kustomize to build the manifests and apply them to your cluster. This will create the `k20s-system` namespace, the CRD, RBAC rules, and the controller `Deployment`.

```bash
make deploy
```

### 3. Verify the Deployment

Check that the controller pod is running in the `k20s-system` namespace:

```bash
kubectl get pods -n k20s-system
```

You should see a pod named `k20s-controller-manager-...` with a status of `Running`.

## Next Steps

Now that the controller is running, you can learn how to use it to optimize your applications.