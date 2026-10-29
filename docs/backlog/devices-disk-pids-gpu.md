# More devices: disk, PIDs/FDs, GPU

Tags: chaos, container · Effort: M

## Approach
- Disk: fill volume to N%; remount read-only; block-device throttle via cgroup `io.max` (true latency/IOPS caps).
- Exhaust `pids.max`; low `nofile` ulimit.
- GPU (stretch): clock caps via `nvidia-smi`, or hide the device.

## Open questions
- cgroup v1 vs v2 coverage.
- Cleanup guarantees for filled volumes.
