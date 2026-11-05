# Authentication dependency outage rehearsal

This example drops replies from one authentication server to one dedicated
staging container. Run it only against systems you are authorized to test.
It defaults to dry-run; it does not set production targets, credentials, or
server addresses. `--probability 1.0` blocks all matching packets; smaller
values such as `0.5` are also supported for partial loss.

## Prepare and run

Choose a dedicated staging auth-server IPv4 address, one exact staging target
name or ID, and a run ID you have not used for another active run. Set
`AUTH_PORTS` to the reply service port(s):

| Service                          | Protocol | Reply port(s) |
| -------------------------------- | -------- | ------------- |
| TACACS+                          | TCP      | `49`          |
| RADIUS authentication/accounting | UDP      | `1812,1813`   |
| LDAP                             | TCP      | `389`         |
| LDAPS                            | TCP      | `636`         |

The script matches inbound reply packets by both the auth server's source IP
and source port. `--match-all` makes those dimensions AND conditions. The
protocol must be TCP or UDP. `MAX_TARGETS` defaults to `1`; the script also
uses a bounded duration (30 seconds by default, maximum 10 minutes) and a
15-minute `--max-duration` budget. Review `examples/auth_outage_rehearsal.sh`
for the accepted environment variables and limits.

```sh
AUTH_IP=192.0.2.15 AUTH_PORTS=49 TARGET=staging-app-1 RUN_ID=auth-drill-2026-01 \
  PROTOCOL=tcp ./examples/auth_outage_rehearsal.sh
```

This is a dry-run. Inspect the emitted NDJSON plan and confirm the selected
target and selectors. To apply the outage, repeat with `APPLY=1`:

```sh
APPLY=1 AUTH_IP=192.0.2.15 AUTH_PORTS=49 TARGET=staging-app-1 \
  RUN_ID=auth-drill-2026-01 PROTOCOL=tcp \
  ./examples/auth_outage_rehearsal.sh
```

The script runs Pumba in the foreground via `exec`. It does not background a
chaos process or claim that the fault became active just because the command
started. Keep the NDJSON stream. In a second terminal, wait for a
`target.applied` event for every intended target before testing authentication.
If the event is absent, dropped, or the run fails before it arrives, do not
claim the outage was active or interpret the auth test as an outage result.
Exercise only the staging workload while the bounded run is active.

Check the terminal `target.result`, the final `run.end`, and Pumba's exit code.
After the run, verify current fault state (using the runtime and target covered
by the run), for example:

```sh
pumba status --check staging-app-1
```

`status --check` reports a verdict, not a guarantee that every application
request recovered. A clean verdict and successful `run.end` are useful
verification evidence; inspect the workload and authentication service too.
If recovery fails or status is dirty/unknown, stop further experiments and
investigate before retrying. `--max-duration` bounds the run but is not a
promise against process kill, host/runtime failure, or failed cleanup. Preserve
events and logs for diagnosis.

## Degraded-path alternative

For a staging container with supported egress, combine destination IP and
service destination port using AND-scoped netem filters. This changes outbound
requests, not inbound replies:

```sh
pumba --run-id auth-slow-2026-01 --max-duration 15m --max-targets 1 \
  --output ndjson netem --match-all --duration 5m \
  --target 192.0.2.15 --ingress-port 49 \
  delay --time 2500 --jitter 500 staging-app-1
```

For loss, use the same parent selectors and replace the action with
`loss --percent 50 staging-app-1`. Netem qdiscs are not tagged with run IDs and
`pumba revert` does not remove qdiscs. If netem cleanup fails, use
`pumba status --check` and follow the documented manual qdisc recovery
procedure; do not assume run-scoped revert can repair it.

## Offline checks

The example has a fake-Pumba boundary test and requires no container runtime:
`bash tests/unit/auth_outage_rehearsal_test.bash` (or `make test-shell`).
