# How to Use K20s

Using K20s involves creating a `ResourceOptimizerProfile` custom resource to define your optimization strategy for a target workload.

## 1. Deploy a Sample Application

First, let's deploy a sample Nginx application that our controller can manage.

```yaml
# sample-app.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx-deployment
  labels:
    app: my-nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-nginx
  template:
    metadata:
      labels:
        app: my-nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.21
        resources:
          requests:
            cpu: "100m"
```

Apply it to your cluster:
```bash
kubectl apply -f sample-app.yaml
```

## 2. Create a ResourceOptimizerProfile

Now, create a `ResourceOptimizerProfile` to target the Nginx application. You can choose one of three policies.

### Option A: `Recommend` Policy (Safest)

This policy analyzes the workload and provides a recommendation in the resource's `.status` field without taking any action.

```yaml
# recommend-profile.yaml
apiVersion: optimizer.example.com/v1
kind: ResourceOptimizerProfile
metadata:
  name: my-nginx-recommend
spec:
  selector:
    matchLabels:
      app: my-nginx
  optimizationPolicy: Recommend
  cpuThresholds:
    min: 20
    max: 60
```

### Option B: `Scale` Policy

This policy will automatically scale the number of replicas up or down.

```yaml
# scale-profile.yaml
apiVersion: optimizer.example.com/v1
kind: ResourceOptimizerProfile
metadata:
  name: my-nginx-scale
spec:
  selector:
    matchLabels:
      app: my-nginx
  optimizationPolicy: Scale
  cpuThresholds:
    min: 20
    max: 60
  cooldownPeriod: "2m" # Optional: wait 2 minutes between actions
```

### Option C: `Resize` Policy

This policy will automatically adjust the CPU requests of the pods themselves.

```yaml
# resize-profile.yaml
apiVersion: optimizer.example.com/v1
kind: ResourceOptimizerProfile
metadata:
  name: my-nginx-resize
spec:
  selector:
    matchLabels:
      app: my-nginx
  optimizationPolicy: Resize
  cpuThresholds:
    min: 20
    max: 60
  cooldownPeriod: "5m"
  minCPU: "50m"   # Safety rail: never set request below 50m
  maxCPU: "500m"  # Safety rail: never set request above 500m
```

Apply the profile of your choice:
```bash
kubectl apply -f recommend-profile.yaml
```

## 3. Observe the Status

You can view the controller's actions and analysis on the built-in status dashboard.

```bash
# First, port-forward to the controller's service
kubectl port-forward svc/k20s-controller-manager-metrics-service 8080:8080 -n k20s-system
```

Now, open your browser to **http://localhost:8080/status** to see a live view of all profiles.