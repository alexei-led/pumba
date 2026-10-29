# Clock skew and syscall fault injection

Tags: chaos · Effort: L

## Why
Tests TLS expiry, JWT validation, leases, and error handling for disk/network syscalls. Little equivalent tooling in the Docker ecosystem.

## Approach
- Time: `libfaketime` via LD_PRELOAD, or time namespace (`CLONE_NEWTIME`, Linux 5.6+).
- Syscalls: eBPF `bpf_override_return` or seccomp `SECCOMP_RET_ERRNO` sidecar making `open`/`write`/`connect` fail with EIO/ENOSPC/ECONNREFUSED.

## Open questions
- Kernel/privilege requirements; behavior on Colima and podman machine.
- Injecting into already-running processes.
