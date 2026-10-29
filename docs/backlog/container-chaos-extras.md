# Container chaos extras

Tags: container · Effort: S–M each

## Approach
- OOM-kill simulation: SIGKILL with exit 137 and matching event.
- Crashloop: repeated kill with backoff.
- Freeze one process (SIGSTOP by name) rather than the whole container.
- Image pull failure: block registry egress, restart container.
- Healthcheck sabotage: fail healthcheck without breaking the app.

## Open questions
- Which are worth a command vs a documented recipe with `exec`/`iptables`.
