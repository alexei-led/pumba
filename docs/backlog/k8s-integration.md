# Kubernetes integration

Tags: k8s · Effort: M (plugin) / L (controller)

## Approach
- First: `kubectl pumba` plugin that execs into the DaemonSet pod on the target pod's node. Most value for least cost.
- Native selection: `--k8s-namespace ns --k8s-selector app=web`; resolve pods via API, map to container IDs on the node.
- Respect PodDisruptionBudgets for lifecycle chaos.
- Write k8s Events on targeted pods.
- Later: `PumbaExperiment` CRD + controller fanning out to DaemonSet pods (competes with Chaos Mesh/Litmus; big scope decision).

## Open questions
- RBAC model for the plugin and DaemonSet.
