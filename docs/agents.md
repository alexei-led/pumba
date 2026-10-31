# Pumba for agents and scripts

How a program runs Pumba and reads what happened, without parsing logs.

- `--output ndjson`: result events on stdout, one JSON object per line (schema `pumba.event/v1`).
- `--run-id`: names the run; every event carries it, and it tags the sidecars and iptables rules the run creates.
- `pumba status --check`: did a run leave faults behind? A `clean`/`dirty`/`unknown` verdict.
- `--max-duration`: a time budget, so a forgotten `--interval` run stops by itself.
- The exit code is `0` or `1`. The detail is in the last event, `run.end`.

Logs stay on stderr in every mode. `--json` only changes the log format; it combines freely with `--output ndjson`.

## Quick start

```bash
pumba -o ndjson --run-id ci-42 netem --duration 30s delay --time 100 web-1 2>pumba.log \
  | tee events.ndjson
tail -n1 events.ndjson | jq -r .status   # ok, failed, partial, cleanup_failed, ...
```

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--output`, `-o` | `text` | `text`: chaos logs only, stdout empty; `status` prints its table and optional verdict to stdout. `ndjson`: events on stdout; help and usage text move to stderr |
| `--run-id` | generated | 1–48 characters from `[A-Za-z0-9_.-]`. Generated as 12 hex characters when empty. An invalid value is a usage error |
| `--max-duration` | `0` (unlimited) | Time budget for active chaos and `--interval` scheduling. See [Run budget](#run-budget---max-duration) |

One run ID per process: every `--interval` tick shares it.

## Event stream

Every event has these fields:

| Field | Meaning |
|---|---|
| `schema` | `pumba.event/v1` |
| `type` | `run.start`, `target.applied`, `target.result`, `run.end` |
| `run_id` | the `--run-id` value or the generated ID |
| `time` | RFC 3339, UTC |
| `action` | command path, e.g. `kill`, `netem delay`, `iptables loss` |

Order: one `run.start`, then per tick and target `target.applied` (fault active) and `target.result` (terminal), then one `run.end`.

```json
{"schema":"pumba.event/v1","type":"run.start","run_id":"demo-1","time":"2026-10-30T06:44:42.077Z","action":"netem delay","pumba_version":"1.5.0","runtime":"docker","dry_run":false,"interval":"0s","selection":{"names":["web-1"],"labels":[],"random":false,"max_targets":2},"params":{"correlation":20,"distribution":"","duration":"3s","egress-port":"","ingress-port":"","interface":"eth0","jitter":10,"limit":0,"pull-image":false,"target":[],"tc-image":"ghcr.io/alexei-led/pumba-alpine-nettools:latest","time":100}}
{"schema":"pumba.event/v1","type":"target.applied","run_id":"demo-1","time":"2026-10-30T06:44:42.259Z","action":"netem delay","tick":1,"target":{"id":"b248a75b71ec…","name":"/web-1"}}
{"schema":"pumba.event/v1","type":"target.result","run_id":"demo-1","time":"2026-10-30T06:44:45.419Z","action":"netem delay","tick":1,"target":{"id":"b248a75b71ec…","name":"/web-1"},"started":"2026-10-30T06:44:42.136Z","ended":"2026-10-30T06:44:45.419Z","status":"ok","error":null,"cleanup":{"ok":true}}
{"schema":"pumba.event/v1","type":"run.end","run_id":"demo-1","time":"2026-10-30T06:44:45.419Z","action":"netem delay","status":"ok","counts":{"selected":1,"ok":1,"failed":0,"cleanup_failed":0,"planned":0,"skipped":0},"error":null,"events_dropped":0}
```

### `run.start`

Emitted once flags are parsed and the command is built.

- `pumba_version`, `runtime`, `dry_run`, `interval` (Go duration string).
- `selection`: `names`, `pattern` (only for a `re2:` selector), `labels`, `random`, `max_targets` (`0`: no cap).
- `params`: the action's typed flag values, including flags set on parent commands (`netem`, `iptables`). Global flags are not included. `exec --args` is left out because it may carry secrets.

### `target.applied`

The fault is active on the target, so probes can start now. For one-shot actions (`kill`, `rm`, `restart`, `exec`, `stop` without `--restart`) it means the action ran. Fields: `tick`, `target{id,name}`. Not emitted in dry-run.

### `target.result`

Terminal record per target and tick. For faults that are reverted (netem, iptables, pause, stop `--restart`, stress), it comes only after the revert finished.

| Field | Meaning |
|---|---|
| `tick` | interval iteration, starting at 1 |
| `target` | `id` and `name` as the runtime reports them (Docker names start with `/`) |
| `started`, `ended` | when work on this target started and finished |
| `status` | `ok`, `failed`, `cleanup_failed`, `planned` (dry-run), `skipped` (serial run stopped by an earlier failure) |
| `error` | `null` or an [error object](#errors) |
| `cleanup` | `null` when nothing was reverted (one-shot actions, dry-run); else `{"ok":true}` or `{"ok":false,"error":"…"}` |
| `state` | `status` only: what was found on the target, see [Status](#status-pumba-status---check) |

### `run.end`

Emitted when the process finishes, also for usage errors that never reached `run.start`.

- `status`: see the table below.
- `counts`: `selected` (number of `target.result` events), `ok`, `failed`, `cleanup_failed`, `planned`, `skipped`.
- `error`: `null` or an [error object](#errors).
- `events_dropped`: events lost on the way to stdout (see [Delivery](#delivery)).
- `coverage`: `status` only, see [Status](#status-pumba-status---check).

| `status` | When | Exit |
|---|---|---|
| `ok` | every target `ok` or `planned` | 0 |
| `no_targets` | nothing matched; `error.code` is `no_targets` | 0 |
| `failed` | an error, and no target succeeded | 1 (0 with `--skip-error`) |
| `partial` | an error, and some targets succeeded | 1 (0 with `--skip-error`) |
| `cleanup_failed` | a revert failed; the fault may still be active | 1 (0 with `--skip-error`) |
| `refused` | `--max-targets` exceeded; nothing was injected | 1 |
| `usage_error` | bad flag, argument or `--run-id` | 1 |
| `aborted` | `--max-duration` expired (`timeout`, exit 1) or SIGINT/SIGTERM (`canceled`, exit 0) | see left |
| `clean`, `dirty`, `unknown` | `status --check` only: the verdict | 0 for `clean`, else 1 |

For chaos commands, the first matching row wins, top to bottom in this order: `usage_error`, `refused`, `cleanup_failed`, `aborted` (timeout), `failed`/`partial`, `aborted` (canceled), `no_targets`, `ok`. A cleanup failure outranks a timeout because a leftover fault matters more.

When a usage error stops Pumba before `run.start`, `run.end` has `"action":""`. Its `run_id` is the `--run-id` value (or a generated one), and `""` when `--run-id` itself was invalid.

### Errors

`error` is `{"code","phase","message"}`. Branch on `code`; `message` is for humans and may change.

| `code` | `phase` | Meaning |
|---|---|---|
| `invalid_argument` | `usage` | bad flags or arguments |
| `no_targets` | `select` | no container matched |
| `max_targets_exceeded` | `select` | selection is larger than `--max-targets` |
| `runtime_unavailable` | `runtime`, `select` | cannot create the runtime client, or listing containers failed |
| `command_failed` | `inject` | the chaos action failed |
| `command_failed` | `inspect` | `status` could not read a target's netns |
| `cleanup_failed` | `cleanup` | reverting the fault failed |
| `timeout` | `runtime` | `--max-duration` expired |
| `canceled` | `runtime` | SIGINT/SIGTERM |
| `rootless_unsupported` | — | reserved; rootless Podman failures currently report `command_failed` |

## Exit codes

For chaos commands in every output mode, `1` means the command failed (unless suppressed by `--skip-error`). The exit code is authoritative even when `run.end` was lost.

- No targets exits `0` for chaos commands.
- A cleanup failure (netem or iptables rules not removed) exits `1`.
- `--skip-error` hides chaos failures from the exit code, not from the events; refusals and usage errors still exit `1`.
- [`status --check`](#status-pumba-status---check) exits `0` only for `clean`, even with `--skip-error`. Dirty findings need not be command errors, and an empty selection is `unknown` (exit `1`).
- In text mode an unknown command exits `3` (urfave/cli). In ndjson mode it is a usage error and exits `1`.

## Delivery

- Events are counted before delivery, so a slow or closed stdout never changes the outcome or the exit code.
- The stdout writer has a bounded queue (1024 events). When the reader falls behind, events are dropped instead of stalling chaos; `run.end.events_dropped` counts them.
- `run.end` is flushed for at most 2 seconds at exit. A missing `run.end` means the output is incomplete (crash, `kill -9`, closed pipe): treat the outcome as unknown and use the exit code.
- SIGPIPE is ignored in ndjson mode, so a closed stdout does not kill Pumba with faults still active.

## Run budget (`--max-duration`)

The budget starts before the first tick. When it expires, Pumba stops admitting new ticks and fault execs and requests cancellation of in-flight work. Already-admitted tc/iptables execs may continue under their separate two-minute timeout before cleanup. The run ends with `run.end` status `aborted`, code `timeout`, exit `1`.

Limits:

- It works only while Pumba is alive: it does not protect against `kill -9`.
- It does not cap total wall time: admitted fault execs and cleanup can run past the budget.
- It does not undo `kill`, `rm`, or `stop` without `--restart`.

## Interactions

| Flag | Effect on events |
|---|---|
| `--interval` | one `run.start` and `run.end` per process; `target.*` per target per tick |
| `--random`, `--limit` | only selected targets produce events |
| `--max-targets` | refusal: no target events, `run.end` `refused` |
| `--dry-run` | `target.result` `planned`, no `target.applied`; returns without waiting for `--duration` |
| `--skip-error` | chaos failures still recorded in events; exit `0` (not refusals, usage errors or a non-clean `status --check` verdict) |
| SIGINT/SIGTERM | in-flight targets finish cleanup and emit `target.result`; `run.end` `aborted` unless something failed |

`--help` and `--version` are not runs and emit no events. A top-level `pumba --help` still prints to stdout.

## Status (`pumba status [--check]`)

`pumba status [--run-id ID] [--check] [--tc-image IMG] [--pull-image] [containers…]` reports the Pumba faults left on running containers: iptables rules tagged `pumba:<run-id>`, qdiscs under Pumba's root handle `504d:`, and sidecars labeled `com.gaiaadm.pumba.*`. Selection works as for chaos commands; no containers means all running. `--max-targets` applies. `--random` and `--dry-run` are ignored. The status `--run-id` is a **filter** for rules and sidecars; the global `--run-id` only names the status run.

Each inspected container gets one `target.result` with `state`:

```json
{"rules":[{"run_id":"ci-42","chain":"INPUT","spec":"-A INPUT -i eth0 -m comment --comment \"pumba:ci-42\" -m statistic --mode random --probability 1.00000000000 -j DROP"}],
 "qdiscs":[{"device":"eth0","kind":"netem","ownership":"pumba-owned","run_id":null}],
 "sidecars":[{"id":"f965…","name":"/vigilant_mahavira","kind":"stress","state":"running","target_id":"72c5…","run_id":"ci-42"}]}
```

- `qdiscs[].ownership`: `pumba-owned` (Pumba's topology) or `suspicious` (Pumba's handle, other topology). qdiscs have no comment field: `run_id` is always `null` and the run filter never hides them.
- `sidecars[].run_id` is `null` for an untagged sidecar. Status's own `inspect` sidecars are never reported.
- A failed inspection makes the target `failed` with `error.phase` `inspect`; its `state` then holds only sidecars.

`run.end.coverage` holds the verdict and what belongs to no inspected target:

| Field | Meaning |
|---|---|
| `verdict` | `clean`, `dirty` or `unknown`; present with or without `--check` |
| `reasons` | why `dirty` or `unknown`; `[]` when clean |
| `run_filter` | the status `--run-id`, omitted when unset |
| `orphan_sidecars` | sidecars whose target was not inspected (gone or stopped); only when the selection is empty: no names, no `re2:` pattern and no `--label` |
| `missing_targets` | named containers that do not exist or are not running |
| `listing_gaps` | containers the runtime could not read (`id`, `error`) |

| Verdict | When |
|---|---|
| `clean` | every selected target was inspected, coverage has no gaps, nothing matched |
| `dirty` | a tagged rule or labeled sidecar matched (or an orphan sidecar); without a run filter, also a `pumba-owned` qdisc |
| `unknown` | a listing or an inspection failed, a target is missing, a listing gap, a `suspicious` qdisc, nothing selected, or with a run filter only qdiscs found |

`dirty` wins over `unknown`. `clean` is never vacuous: an empty selection is `unknown`.

With `--check`, the verdict is the `run.end.status` and the exit code: `0` only for `clean`, also with `--skip-error`. A real error stays in `run.end.error`. A failed listing gives `unknown` unless already discovered sidecars establish `dirty`; either way it includes the reason `listing failed: ...`. A refused (`--max-targets`) or usage error keeps its own status. `--check` with `--interval` is a usage error. Without `--check`, `status` exits like any command (`1` only when an inspection or the listing failed) and the verdict is only in `coverage`.

In text mode `status` prints a `TARGET KIND DETAIL RUN` table to stdout, plus a `verdict:` line with `--check`. In ndjson mode stdout carries only events.

Known limits: iptables rules from Pumba 1.4 and earlier have no comment; containerd stress with `--stress-image ""` leaves no container; containers sharing a netns report the same rules and qdiscs. Rootless Podman is refused, as for netem.

## Recipes

Wait until the fault is active, then probe:

```bash
pumba -o ndjson pause --duration 30s web-1 2>/dev/null | while read -r ev; do
  case $(jq -r .type <<<"$ev") in
    target.applied) ./probe.sh & ;;
    run.end) jq -r '"status=\(.status) code=\(.error.code // "-")"' <<<"$ev" ;;
  esac
done
```

Fail a CI step on anything but a clean run:

```bash
pumba -o ndjson netem --duration 20s loss --percent 10 web-1 2>pumba.log >events.ndjson
tail -n1 events.ndjson | jq -e '.type == "run.end" and .status == "ok"'
```

Preview targets without touching anything:

```bash
pumba -o ndjson --dry-run kill 're2:^web-' | jq -r 'select(.type=="target.result") | .target.name'
```

Check that a run left nothing behind, even after a crash:

```bash
pumba -o ndjson --run-id ci-42 iptables --duration 5m loss --probability 0.2 web-1 >/dev/null &
# ... the run is killed with -9 ...
pumba -o ndjson status --check --run-id ci-42 web-1 | tail -n1 | jq '{status, reasons: .coverage.reasons}'
```

Bound a recurring run:

```bash
pumba -o ndjson --max-duration 10m --interval 1m kill --signal SIGTERM 're2:^worker-'
```

See also [examples/pumba_agent_output.sh](../examples/pumba_agent_output.sh).

## Versioning

Within `pumba.event/v1`, fields and event types may be added; ignore what you do not know. Renaming, removing or re-meaning a field, or adding a `status` or `code` value, bumps the schema to `v2`.
