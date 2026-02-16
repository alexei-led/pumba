# Deployment Guide

This guide covers how to run Pumba in Docker containers, on Kubernetes clusters, and on OpenShift. For general usage, see the [User Guide](guide.md).

## Running as a Docker Container

Pumba is distributed as a minimal `scratch` Docker image containing only the `pumba` binary with `ENTRYPOINT` set to the `pumba` command.

### GHCR (Recommended)

```bash
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock \
    ghcr.io/alexei-led/pumba --interval=10s --random kill --signal=SIGKILL "re2:^test"
```

### Docker Hub (Deprecated)

```bash
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock \
    gaiaadm/pumba --interval=10s --random kill --signal=SIGKILL "re2:^test"
```

### Docker Socket Access

Pumba needs access to the Docker daemon socket to manage containers:

- **Linux**: Mount `/var/run/docker.sock` as shown above
- **Windows/macOS**: Use the `--host` flag to specify the Docker daemon address, since there is no Unix socket to mount

### Example: Kill Containers by Pattern

```bash
# Start some test containers
for i in $(seq 1 10); do docker run -d --name test_$i --rm alpine tail -f /dev/null; done

# Kill matching containers every 10 seconds
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock \
    ghcr.io/alexei-led/pumba \
    --interval=10s --random --log-level=info \
    kill --signal=SIGKILL "re2:^test"
```

## Running on Kubernetes

Pumba works well with Kubernetes DaemonSets, which automatically deploy Pumba to selected nodes.

### Deploying with DaemonSet

```sh
kubectl create -f deploy/pumba_kube.yml
```

The [deploy/](../deploy/) directory contains ready-to-use manifests:

- `pumba_kube.yml` — DaemonSet with pause and netem delay examples
- `pumba_kube_stress.yml` — DaemonSet for stress testing
- `pumba_openshift.yml` — OpenShift DaemonSet

### Node Selection

Use `nodeSelector` or `nodeAffinity` to target specific nodes. See [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).

```yaml
spec:
  template:
    spec:
      # EKS node group
      nodeSelector:
        alpha.eksctl.io/nodegroup-name: my-node-group
      # Or GKE node pool
      # nodeSelector:
      #   cloud.google.com/gke-nodepool: node-pool
```

### Container Label Filtering

Kubernetes automatically assigns labels to Docker containers. Use Pumba's `--label` flag to target specific Pods and Namespaces:

```yaml
# Available K8s labels for filtering
"io.kubernetes.container.name": "test-container"
"io.kubernetes.pod.name": "test-pod"
"io.kubernetes.pod.namespace": "test-namespace"
```

### Multiple Pumba Commands

Run multiple chaos commands in the same DaemonSet by defining multiple containers:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: pumba
spec:
  selector:
    matchLabels:
      app: pumba
  template:
    metadata:
      labels:
        app: pumba
        com.gaiaadm.pumba: "true" # prevent pumba from killing itself
      name: pumba
    spec:
      containers:
        # Pause containers in a specific Pod
        - image: ghcr.io/alexei-led/pumba
          name: pumba-pause
          args:
            - --random
            - --log-level
            - info
            - --label
            - io.kubernetes.pod.name=test-1
            - --interval
            - 20s
            - pause
            - --duration
            - 10s
          securityContext:
            capabilities:
              add: ["NET_ADMIN"]
          resources:
            requests:
              cpu: 10m
              memory: 5M
            limits:
              cpu: 100m
              memory: 20M
          volumeMounts:
            - name: dockersocket
              mountPath: /var/run/docker.sock
        # Add network delay to a different Pod
        - image: ghcr.io/alexei-led/pumba
          name: pumba-delay
          args:
            - --random
            - --log-level
            - info
            - --label
            - io.kubernetes.pod.name=test-2
            - --interval
            - 30s
            - netem
            - --duration
            - 20s
            - --tc-image
            - ghcr.io/alexei-led/pumba-debian-nettools
            - delay
            - --time
            - "3000"
            - --jitter
            - "30"
            - --distribution
            - normal
          resources:
            requests:
              cpu: 10m
              memory: 5M
            limits:
              cpu: 100m
              memory: 20M
          volumeMounts:
            - name: dockersocket
              mountPath: /var/run/docker.sock
      volumes:
        - hostPath:
            path: /var/run/docker.sock
          name: dockersocket
```

### Self-Protection

Add the label `com.gaiaadm.pumba: "true"` to the Pumba Pod to prevent it from killing itself.

### Stress Testing on Kubernetes

For stress testing, the Pumba container no longer needs `SYS_ADMIN` capability, but it must be able to create containers with the correct cgroup parent.

```yaml
- image: ghcr.io/alexei-led/pumba
  name: pumba-stress
  args:
    - --log-level
    - debug
    - --label
    - io.kubernetes.pod.name=test-stress
    - --interval
    - 2m
    - stress
    - --duration
    - 1m
```

See `deploy/pumba_kube_stress.yml` for a complete example.

### Limitations

- `pumba netem` commands do not work on minikube because the `sch_netem` kernel module is missing in the minikube VM

## Running on OpenShift

Pumba can be deployed on OpenShift using a DaemonSet similar to Kubernetes. See `deploy/pumba_openshift.yml` for an example manifest.

```sh
oc create -f deploy/pumba_openshift.yml
```

The OpenShift manifest uses `runAsUser: 0` to ensure Pumba has the necessary permissions to interact with the Docker socket.

## Related Documentation

- [User Guide](guide.md) — Container chaos commands and targeting
- [Network Chaos](network-chaos.md) — netem and iptables commands
- [Stress Testing](stress-testing.md) — CPU, memory, and I/O stress tests
