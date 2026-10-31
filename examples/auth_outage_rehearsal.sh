#!/bin/sh
#
# auth_outage_rehearsal.sh - auth-dependency outage rehearsal runbook.
#
# Mirrors docs/auth-outage-rehearsal.md: blacks out the auth server's reply
# traffic to your containers with pumba's iptables selectors, then runs a
# degraded-path variant with netem delay/loss. No hand-written iptables
# rules needed - --source already exists on the iptables command.
#
# Auth and login checks run WHILE each chaos phase is active: pumba starts
# in the background, you exercise logins during that window, then the phase
# is stopped (pumba removes its rules on stop/abort; SIGTERM triggers the
# abort path). Nothing is exercised after the phase ends - post-phase
# checks can only see the recovered state.
#
# Gating: the script aborts unless the dry run succeeds AND you type an
# explicit 'yes' confirmation. A failed read (stdin exhausted/redirected)
# also aborts - no disruption happens without interactive approval.
#
# All knobs are environment variables:
#
#   AUTH_IP     IP of the auth server (TACACS+/RADIUS/LDAP/AD)
#   AUTH_PROTO  tcp for TACACS+/LDAP/AD, udp for RADIUS        (default: tcp)
#   AUTH_PORTS  comma-separated service ports, for reference    (default: 49)
#               TACACS+ 49 | RADIUS 1812,1813 | LDAP 389,636
#               AD Kerberos 88 | kpasswd 464 | GC 3268,3269
#               Shown in the plan line only: the iptables runs scope with
#               --source alone, because every selector installs its own
#               DROP rule (OR semantics) - stacking --source with
#               --src-port would widen the blast radius, not narrow it.
#               Same for netem: each filter flag installs its own tc
#               filter, so the netem phases use --target alone.
#   TARGET      target containers (name or re2: regex)           (default: re2:^web-)
#   DURATION    how long each chaos phase lasts                   (default: 5m)
#   TC_IMAGE    netem sidecar image                               (default: ghcr.io/alexei-led/pumba-debian-nettools)
#
# Example: rehearse a RADIUS outage against the VPN gateway containers:
#   AUTH_IP=10.0.5.11 AUTH_PROTO=udp AUTH_PORTS=1812,1813 TARGET=re2:^vpn-gateway- ./auth_outage_rehearsal.sh

set -o xtrace

AUTH_IP="${AUTH_IP:-10.0.5.10}"
AUTH_PROTO="${AUTH_PROTO:-tcp}"
AUTH_PORTS="${AUTH_PORTS:-49}"
TARGET="${TARGET:-re2:^web-}"
DURATION="${DURATION:-5m}"
TC_IMAGE="${TC_IMAGE:-ghcr.io/alexei-led/pumba-debian-nettools}"

abort() {
  echo "aborting: $1" >&2
  exit 1
}

# Signal safety: Ctrl-C (SIGINT) or SIGTERM during a phase stops the
# backgrounded pumba run instead of orphaning the disruption until
# --duration expires. PUMBA_PID is cleared after every clean stop so the
# trap never touches a stale pid.
PUMBA_PID=""
on_signal() {
  if [ -n "$PUMBA_PID" ]; then
    kill "$PUMBA_PID" 2>/dev/null || true
    wait "$PUMBA_PID" 2>/dev/null || true
    PUMBA_PID=""
  fi
  exit 130
}
trap on_signal INT TERM

# stop_phase <pid>: end a backgrounded pumba run; pumba removes its
# rules/qdisc on stop (SIGTERM -> abort path -> cleanup). Returns pumba's
# exit status: 0 on a clean stop (cleanup reported OK), nonzero when the
# run or its cleanup errored. A nonzero return means disruption may still
# be installed - callers must NOT proceed to the next phase.
stop_phase() {
  kill "$1" 2>/dev/null || true
  wait "$1"
}

# announce_phase <label> <pid> <log> <marker>: wait for the install marker,
# confirming the disruption is really installed before announcing it:
# the pumba process must stay alive and its log must show the
# per-container install marker. Polls for up to ~60s (cold sidecar image
# pulls are slow) instead of a single fixed sleep. Returns 0 when
# installed, 1 with the log tail otherwise - a phase is never declared
# active on a dead or silent pumba run.
announce_phase() {
  label="$1"; pid="$2"; logfile="$3"; marker="$4"
  tries=0
  while [ "$tries" -lt 30 ]; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "phase '${label}' failed: pumba exited before installing - log tail:" >&2
      tail -20 "$logfile" >&2
      return 1
    fi
    if grep -q "$marker" "$logfile" 2>/dev/null; then
      echo ""
      echo "${label} is ACTIVE. Exercise logins NOW, while the disruption is in place:"
      return 0
    fi
    tries=$((tries + 1))
    sleep 2
  done
  echo "phase '${label}' failed: pumba is alive but never reported install - log tail:" >&2
  tail -20 "$logfile" >&2
  return 1
}

# run_phase <label> <install-marker> <verify-hint> <exercise-lines> -- <pumba args...>
# Full lifecycle of one chaos phase: start pumba in the background with
# its log captured, verify install before announcing, hold an interactive
# window for login exercise, then stop and verify cleanup. Returns 0 on a
# fully clean phase; nonzero aborts the caller - a failed install, a lost
# terminal, or a failed cleanup all stop the rehearsal, never cascade.
run_phase() {
  # $5 is the conventional "--" separator between our args and the pumba
  # command, so shift 5 to leave "$@" as the command to execute.
  label="$1"; marker="$2"; verify_hint="$3"; exercise_lines="$4"; shift 5
  PUMBA_LOG="$(mktemp /tmp/auth-outage-rehearsal.XXXXXX.log)"
  "$@" >"$PUMBA_LOG" 2>&1 &
  PUMBA_PID=$!
  if ! announce_phase "$label" "$PUMBA_PID" "$PUMBA_LOG" "$marker"; then
    # Install never verified: stop the backgrounded run before reporting
    # failure. A slow-starting pumba (e.g. pulling the tc sidecar image)
    # could otherwise install the disruption after we give up and leave
    # it running unsupervised until --duration expires. If the stop
    # itself errors, say so: a rule or qdisc may have survived, and the
    # operator must verify before re-running.
    if ! stop_phase "$PUMBA_PID" 2>/dev/null; then
      echo "warning: stopping the failed '${label}' phase reported an error - ${verify_hint} to confirm no rules/qdisc were left behind before re-running" >&2
    fi
    PUMBA_PID=""
    rm -f "$PUMBA_LOG"
    return 1
  fi
  echo "$exercise_lines"
  echo "Press enter when done (phase ends on its own after ${DURATION})"
  if ! read -r _; then
    if ! stop_phase "$PUMBA_PID" 2>/dev/null; then
      echo "warning: stopping the phase after lost input reported an error - ${verify_hint} to confirm no rules/qdisc were left behind" >&2
    fi
    PUMBA_PID=""
    rm -f "$PUMBA_LOG"
    abort "lost input - phase stopped"
  fi
  if ! stop_phase "$PUMBA_PID"; then
    PUMBA_PID=""
    rm -f "$PUMBA_LOG"
    abort "phase cleanup reported an error - disruption may still be installed; ${verify_hint} and clean up manually before re-running"
  fi
  PUMBA_PID=""
  rm -f "$PUMBA_LOG"
  return 0
}

echo "Rehearsal plan: drop ${AUTH_PROTO}/${AUTH_PORTS} replies from ${AUTH_IP}"
echo "  against containers: ${TARGET}, ${DURATION} per phase"
echo ""

# Phase 0: dry run - verify targeting without touching traffic.
# A failed dry run, a failed read, or a declined confirmation aborts
# everything before any disruption starts.
pumba --dry-run iptables --duration "$DURATION" --protocol "$AUTH_PROTO" \
  --source "$AUTH_IP" \
  loss --probability 1.0 "$TARGET" || abort "dry run failed - refusing to disrupt traffic"

echo "Type 'yes' to ARM the real blackhole (anything else aborts)"
read -r reply || abort "no confirmation received (stdin not interactive?)"
[ "$reply" = "yes" ] || abort "not confirmed"

# Phase 1: full blackhole - drop 100% of the auth server's reply packets.
# NOTE: pumba installs rules on the INPUT chain inside the target container,
# so --source selects the auth server's *replies*, which is what makes the
# dependency look unreachable to the container. A single selector only:
# --source and --src-port would each install their own DROP rule.
if ! run_phase "Blackhole" "running iptables on container" \
  "verify with: docker exec <target-container> iptables -S INPUT" \
  "  - fresh login, token rotation, session re-validation" \
  -- pumba --log-level=info iptables --duration "$DURATION" --protocol "$AUTH_PROTO" \
    --source "$AUTH_IP" \
    loss --probability 1.0 "$TARGET"; then
  abort "blackhole phase failed - fix the error above and re-run"
fi

echo "Blackhole phase done. Verify cleanup inside a target container:"
echo "  docker exec <target-container> iptables -S INPUT"

echo ""
echo "Type 'yes' for the degraded-path variant (slow auth), anything else stops here"
read -r reply || abort "no confirmation received - stopping"
[ "$reply" = "yes" ] || abort "not confirmed"

# Phase 2: degraded path - 2.5s delay + 500ms jitter.
# --target scopes the delay to traffic bound for the auth server (an egress
# filter: inbound replies can't be selected by netem). A single filter only:
# each flag installs its own tc filter (OR semantics), so adding
# --ingress-port alongside --target would ALSO degrade traffic to the auth
# port on every other host - widening the blast radius, not narrowing it.
if ! run_phase "Slow-auth phase" "running netem on container" \
  "verify with: docker exec <target-container> tc qdisc show" \
  "  - note p50/p99 latency, timeouts, fallback behavior" \
  -- pumba --log-level=info netem --duration "$DURATION" --tc-image "$TC_IMAGE" \
    --target "$AUTH_IP" \
    delay --time 2500 --jitter 500 "$TARGET"; then
  abort "slow-auth phase failed - fix the error above and re-run"
fi

echo ""
echo "Type 'yes' for the flaky-auth variant (50% loss), anything else stops here"
read -r reply || abort "no confirmation received - stopping"
[ "$reply" = "yes" ] || abort "not confirmed"

# Phase 3: degraded path - 50% packet loss, scoped to auth-bound traffic.
# Same single-filter rule as Phase 2 (see comment there).
if ! run_phase "Flaky-auth phase" "running netem on container" \
  "verify with: docker exec <target-container> tc qdisc show" \
  "  - retries bounded with backoff, or unbounded hammering?" \
  -- pumba --log-level=info netem --duration "$DURATION" --tc-image "$TC_IMAGE" \
    --target "$AUTH_IP" \
    loss --percent 50 "$TARGET"; then
  abort "flaky-auth phase failed - fix the error above and re-run"
fi

echo ""
echo "Rehearsal complete. Post-run checks:"
echo "  1. iptables rules removed?  docker exec <target> iptables -S INPUT"
echo "  2. auth recovered without restarts? try a fresh login"
echo "  3. fill in docs/auth-outage-rehearsal.md section 5 (expected vs actual)"
