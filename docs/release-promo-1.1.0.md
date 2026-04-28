# Pumba v1.1.0 — Release Promo Posts

**Release:** https://github.com/alexei-led/pumba/releases/tag/1.1.0
**Repo:** https://github.com/alexei-led/pumba
**Angle:** Podman-compatible ≠ Docker-compatible. Release-first (no article this time).

Copy/paste each block. Nothing is shared across platforms — each post is tuned.

---

## 🐦 Twitter/X — Thread

**Tweet 1 (hook):**

```
Podman advertises a Docker-compatible socket.

Point the Docker SDK at it and 90% of calls just work.

I spent this month finding the 10%.

Pumba v1.1.0 ships native Podman support today. Thread on the landmines. 🧵
```

**Tweet 2:**

```
Landmine #1: ContainerExecStart.

  Docker: accepts ExecStartOptions{} — no attach, no detach. Works.
  Podman: rejects it. "must provide at least one stream to attach to."

One API. Four callsites. ~60 mock tests. Same SDK, different answer.
```

**Tweet 3:**

```
Landmine #2: cgroups.

  Docker:  docker-<id>.scope
  Podman:  libpod-<id>.scope
  Podman + systemd: libpod-<id>.scope/container/   ← nested leaf
  Podman, inside the container: 0::/              ← ancestry hidden

Stress-testing means getting this right. I rewrote resolution host-side.
```

**Tweet 4:**

```
Landmine #3: SIGTERM.

tc sidecar uses `tail -f /dev/null` as PID 1. Ignores SIGTERM.

Podman sends SIGTERM on DELETE, waits StopTimeout (10s), then SIGKILLs.

Every netem call: +10s sidecar reap.

Fix: StopSignal="SIGKILL" on the sidecar. 10s → 0s.
```

**Tweet 5 (CTA):**

```
v1.1.0 — three runtimes (docker, containerd, podman) behind one CLI.

Rootful only (rootless is honest future work, not marketing).
macOS: runs inside `podman machine` VM.
Linux: native rootful daemon.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo:    https://github.com/alexei-led/pumba

#podman #kubernetes #chaosengineering #sre #devops
```

---

## 🐦 Twitter/X — Single tweet

```
Pumba v1.1.0 — Podman support shipped.

Lesson learned: "Docker-compatible API" is 90% gift, 10% landmine.
`ContainerExecStart({})` works on Docker, rejected by Podman. Cgroup paths diverge. Sidecar reap takes +10s because `tail -f` ignores SIGTERM.

All fixed. All tested.

https://github.com/alexei-led/pumba

#podman #kubernetes #chaosengineering
```

---

## 🔥 Hacker News — Show HN

**Title:**

```
Show HN: Pumba 1.1 – Chaos testing on Podman, and where Docker-compat quietly lies
```

**First comment (body):**

```
I maintain Pumba — a container chaos CLI (kill/pause/rm, netem delay/loss/corrupt, iptables filter, stress-ng via cgroups). Been around since 2016. v1.0 added containerd. v1.1 adds Podman.

You'd think Podman is a gimme: it speaks a Docker-compatible API on a socket, so the Docker SDK connects and most calls work. That part's true. The interesting part is the 10% that doesn't, because the failure modes are subtle enough to slip past casual testing.

Three worth writing up:

1. ContainerExecStart with empty options.
   Docker accepts `ExecStartOptions{}` — no AttachStdout, no AttachStderr, no Detach. It implicitly syncs via the HTTP hijack. Podman's compat API rejects it: *"must provide at least one stream to attach to."* Same SDK call, different server. Four callsites across tc/iptables/exec-check had to switch to `ContainerExecAttach` + drain + inspect. ~60 mocks needed updating because of flags in ExecOptions that Docker let us skip.

2. Cgroup path divergence.
   Docker puts the target in `docker-<id>.scope`. Podman uses `libpod-<id>.scope` — and under systemd often creates a nested `libpod-<id>.scope/container/` as a libpod init sub-cgroup. cgroup v2's "no internal processes" rule means stress-ng sidecars have to target the nested leaf when it exists. On top of that, Podman's default `cgroupns=private` means `/proc/self/cgroup` inside the container shows `0::/` — no useful ancestry. The resolver reads `/proc/<pid>/cgroup` host-side (pumba's view). Which means pumba must run on the same kernel as the targets. On macOS: inside the `podman machine` VM. Same pattern as containerd-in-Colima.

3. Sidecar reap + SIGTERM.
   Pumba spawns a short-lived sidecar to run tc inside the target netns, then removes it. Sidecars use `tail -f /dev/null` as PID 1. PID 1 ignores SIGTERM by default. Podman's DELETE-with-force sends SIGTERM, waits `StopTimeout` (10s), then SIGKILLs. That's +10s per netem call. Fix: `StopSignal: "SIGKILL"` in the container config.

Related: if pumba gets SIGTERM'd between tc exec and sidecar removal, the sidecar leaks and the qdisc stays on the target's netns. Cleanup now uses `context.WithoutCancel(ctx)` with a 15s budget so the defer actually runs.

Bonus landmine, in the inject-cgroup stress mode: Podman 4.9.x on Ubuntu 24.04 creates `<scope>/container/`, migrates PID 1 out, then `rmdir`s it — mid-write. Resolver's `os.Stat` can pass and the directory is gone when we open `cgroup.procs`. ENOENT. Documented Podman behavior (containers/podman#20910). Podman 5.x is stable. That test sits in `tests/skip_ci/` until cg-inject grows a retry-on-ENOENT.

Things I intentionally didn't do:
- **Rootless**. Detected at client init from `Info.SecurityOptions`; netem/iptables/stress fail fast with a pointer to `podman machine set --rootful` or the rootful systemd unit. Doing rootless right needs slirp4netns/pasta netns handling + user-namespace cgroup math. That's a separate release, not a marketing-shaped hack.
- **Yet another runtime abstraction**. Pumba's Podman client embeds the Docker client and overrides only what diverges (cgroup resolution, rootless guards, socket auto-detect). ~300 lines of Podman-specific Go. The rest is shared.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo:    https://github.com/alexei-led/pumba

Happy to go deeper on any of the above — the cgroup resolver and the exec-attach rewrite were the most interesting parts to get right.
```

---

## 💼 LinkedIn — Personal (story angle)

```
Shipped Pumba v1.1.0 today. Native Podman support alongside Docker and containerd.

Going in, I thought this would be a quiet release. Podman advertises a Docker-compatible API. The Docker SDK connects to Podman's socket. Most calls just work. How hard could it be?

Turns out: the 10% that doesn't work is where you spend the month.

Three things I didn't expect:

→ ContainerExecStart with empty options. Docker accepts it — no flags, no attach, no detach. Podman rejects it with "must provide at least one stream to attach to." Same SDK, different server. Four places in pumba had to switch from ExecStart to ExecAttach-with-drain-and-inspect. About sixty mocks needed updates. An afternoon turned into two days.

→ Cgroup naming. Docker uses docker-<id>.scope. Podman uses libpod-<id>.scope. Under systemd, Podman sometimes nests a "container/" leaf inside the scope — and cgroup v2 forbids processes in internal nodes, so stress tests have to target the nested leaf when it exists. On top of that, Podman's default cgroupns=private hides ancestry from inside the container, so resolution has to happen host-side. Which means pumba has to run on the same kernel as the targets. On macOS: inside the podman machine VM.

→ Sidecar reap. tc sidecars use `tail -f /dev/null` as PID 1. PID 1 ignores SIGTERM. Podman's DELETE sends SIGTERM, waits ten seconds for StopTimeout, then SIGKILLs. Every netem call paid ten seconds for nothing. Fix: StopSignal="SIGKILL" in the sidecar config. Ten seconds to zero.

The bigger lesson, repeated: a "compatible" API is usually more dangerous than a completely different one. With a different API you write a new client. With a compatible one you assume it works and find out in production.

What I intentionally didn't ship: rootless Podman support. Doing it correctly needs slirp4netns/pasta netns handling and user-namespace cgroup math — a whole release of its own. Better to be honest about the scope than glue a half-answer on top.

Release notes: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba

If you're running chaos tests on Podman and hit any of these — or if you want to, but don't know where to start — I'd like to hear from you.

#podman #kubernetes #chaosengineering #sre #devops #platformengineering #opensource
```

---

## 💼 LinkedIn Groups (Kubernetes / DevOps / SRE / Podman)

```
Pumba v1.1.0 shipped today — Podman runtime joins Docker and containerd.

Technical write-up condensed:

Podman's Docker-compat socket gets you 90% of the way for free. The other 10% is where everything interesting lives. A few examples: ContainerExecStart with empty options (Docker accepts, Podman rejects), cgroup path divergence (docker-*.scope vs libpod-*.scope, plus a nested /container leaf under systemd), and sidecar reap taking 10s because `tail -f` as PID 1 ignores SIGTERM and Podman waits the full StopTimeout.

Rootful only — rootless is honest future work, not a hidden caveat.

Worth reading if you run chaos experiments across mixed runtimes, or if you've ever assumed "compatible API" means "identical behavior."

https://github.com/alexei-led/pumba/releases/tag/1.1.0

Curious if others have been bitten by subtle Docker/Podman compat gaps — feels like a pattern worth comparing notes on.
```

---

## 🔴 Reddit — r/kubernetes

**Title:**

```
Shipped Podman support for pumba (chaos testing) — here's where "Docker-compatible" quietly wasn't
```

**Body:**

```
Pumba v1.1.0 is out. For anyone who doesn't know: container chaos CLI — kill/stop/rm, netem delay/loss/corrupt, iptables filter, stress-ng via cgroups. Around since 2016. v1.0 added containerd, v1.1 adds Podman.

Going in, I assumed Podman would be easy: it speaks a Docker-compat API, the Docker SDK connects fine, most calls round-trip correctly. That part is true.

The interesting part is the 10% where "mostly compatible" meets chaos-tool-specific code paths. The landmines:

**1. ContainerExecStart with empty options.** Docker's SDK lets you call `ContainerExecStart(ctx, id, ExecStartOptions{})` — no AttachStdout, no AttachStderr, no Detach. Works via HTTP hijack. Podman's compat API rejects it: *"must provide at least one stream to attach to."* Four callsites in pumba had to switch to `ContainerExecAttach` + drain + inspect. About 60 mocks needed updating because the flags Docker didn't care about now matter.

**2. Cgroup paths.**
- Docker: `docker-<id>.scope`
- Podman: `libpod-<id>.scope`
- Podman + systemd: often nests a `libpod-<id>.scope/container/` leaf
- cgroup v2 forbids processes in internal nodes, so stress sidecars must target the leaf when it exists
- Podman's default `cgroupns=private` hides ancestry — `/proc/self/cgroup` inside the container is `0::/`

Resolution moved host-side (reads `/proc/<pid>/cgroup` from pumba's view). Pumba has to run on the same kernel as the targets. On macOS: inside the `podman machine` VM. Same pattern as containerd-in-Colima.

**3. Sidecar reap.** tc sidecars use `tail -f /dev/null` as PID 1. PID 1 ignores SIGTERM. Podman sends SIGTERM on DELETE, waits `StopTimeout` (10s), then SIGKILLs. Fix: `StopSignal: "SIGKILL"` in the sidecar config.

**4. Sidecar cleanup vs. caller cancellation.** If pumba gets SIGTERM'd during the tc-exec window, the cleanup defer never ran — sidecar leaks, netem qdisc stays on the target netns. Cleanup now uses `context.WithoutCancel(ctx)` with a 15s budget.

**5. Podman 4.9.x inject-cgroup race** (bonus, not fully fixed): Ubuntu 24.04 / Podman 4.9.x creates `<scope>/container/`, migrates PID 1, then rmdirs it mid-write. os.Stat passes, write gets ENOENT. containers/podman#20910. The test lives in tests/skip_ci/ until cg-inject gains retry-on-ENOENT. Podman 5.x is stable.

Rootless is intentionally not supported. Detected at init from `Info.SecurityOptions` and failed fast. Doing it right means slirp4netns/pasta netns handling + user-ns cgroup math — separate release.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba

If you're running chaos on Podman and run into corners I missed, open an issue — I'd rather find gaps than pretend they aren't there.
```

---

## 🔴 Reddit — r/podman

**Title:**

```
Pumba (chaos testing) now supports Podman natively — some compat gotchas I hit along the way
```

**Body:**

```
Maintainer of pumba here. Just shipped v1.1.0 with native Podman runtime support. I spent longer than expected in the Docker-compat seam and thought some of this might be useful to other folks building on Podman's compat socket.

Specific things that bit me:

- **`ContainerExecStart` with zero flags** — Docker accepts, Podman rejects ("must provide at least one stream"). Had to switch to `ContainerExecAttach` + drain + `ContainerExecInspect`.
- **Rootless detection**: `Info.SecurityOptions` contains `"rootless"` when applicable. Pumba uses this to fail fast on features that need SYS_ADMIN / raw netns access.
- **Cgroup scope naming**: `libpod-<id>.scope`, sometimes with a nested `container/` leaf under systemd (cgroup v2 internal-processes rule). Resolving requires reading `/proc/<pid>/cgroup` host-side because `cgroupns=private` is the default.
- **Sidecar StopTimeout**: `tail -f` as PID 1 ignores SIGTERM, so Podman waits the full 10s before SIGKILL. `StopSignal: "SIGKILL"` gets you immediate reap.
- **Podman 4.9.x `/container` migration race** for anyone writing into `cgroup.procs` from outside — containers/podman#20910. Stable on 5.x.

Socket auto-detection order (useful if anyone's building similar tooling): `$CONTAINER_HOST`, `$PODMAN_SOCK`, `podman machine inspect`, `/run/podman/podman.sock`, `$XDG_RUNTIME_DIR/podman/podman.sock`.

Rootful only for now. Rootless is planned but it's a real project (slirp4netns/pasta namespaces, user-ns cgroups) and I'd rather not half-ship it.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba

Pointers or corrections welcome. Would especially like to hear from anyone running Podman on Kubernetes (via CRI-O or otherwise) who'd want me to validate pumba there.
```

---

## 🔴 Reddit — r/devops

**Title:**

```
Adding Podman to a chaos CLI that already supported Docker and containerd — the "compatible" API was the hard part
```

**Body:**

```
Shipped pumba v1.1.0 today (container chaos tool — kill, netem, iptables, stress). It now speaks Docker, containerd, and Podman natively.

Going in, I expected Podman to be the easy one. It advertises a Docker-compatible API, the Docker SDK connects fine to its socket, and for common calls the behavior matches. The part I didn't expect: the 10% of calls where Podman's compat API differs are exactly the ones chaos tooling depends on.

A few examples in case they save someone else the discovery cost:

- `ContainerExecStart` with empty `ExecStartOptions{}` works on Docker (implicit HTTP hijack) and is rejected by Podman. Chaos tooling runs a lot of short-lived execs. Pumba now uses `ExecAttach` + drain everywhere.
- Cgroup scope naming differs (`docker-*.scope` vs `libpod-*.scope`, with a nested `/container` leaf under systemd). Stress-ng-in-target's-cgroup needs this right.
- Sidecar cleanup: Podman's `StopTimeout` default is 10s, and `tail -f` as PID 1 ignores SIGTERM, so naive cleanup takes ten seconds per netem call. One-line fix once you know (`StopSignal: "SIGKILL"`).

Rootless is intentionally not yet supported — detected and failed fast with a pointer to the rootful systemd unit / `podman machine set --rootful`.

Broader observation: "compatible API" is usually harder to integrate against than a completely different API, because you don't build the mental model of "this is a different thing." You assume parity, and find the exceptions one by one.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba
```

---

## 🔴 Reddit — r/selfhosted

**Title:**

```
Chaos-test your self-hosted stack on Podman — pumba 1.1 just added native support
```

**Body:**

```
If you're running a Podman-based setup (rootful) and you've ever wondered how your services actually behave when something flakes — pumba v1.1 now supports Podman alongside Docker and containerd.

Concretely, you can:
- Kill / stop / pause / remove containers on a pattern or label
- Inject network delay, packet loss, corruption, duplication
- Stress CPU / memory / IO inside a specific container
- Block traffic with iptables filter rules
- Run on an interval for continuous low-grade chaos

No operator, no daemon. Single static binary. Points at the Podman Docker-compat socket (auto-detected) and goes.

Rootful only at the moment — rootless support is planned but not done. If you're running rootless, this won't work for you yet (and pumba will tell you so at startup instead of failing in weird ways).

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba

Good candidate for trying in staging before you let it near anything you actually depend on.
```

---

## 🔴 Reddit — r/sysadmin

**Title:**

```
Container chaos tool now works on Podman — good for deliberately breaking your own services in a controlled way
```

**Body:**

```
Quick note for anyone running a Podman-based container host who's been curious about chaos engineering: pumba v1.1 now supports Podman natively.

What it does: deliberately breaks containers — kills them, injects network delay/loss, stresses CPU/memory, blocks traffic with iptables — on a schedule or by pattern. Point is to find out how your stack behaves *before* something real goes wrong.

No operator, no config file. Single binary. Works against Podman's Docker-compat socket (auto-detected).

Rootful only right now. Rootless is planned.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba
```

---

## 💬 Lobste.rs

**Title:**

```
Pumba 1.1 — chaos testing on Podman, and the quiet lies of a "Docker-compatible" API
```

**Tags:** `release`, `go`, `linux`, `distributed`

**Body (post-publish comment):**

```
Author here. Three concrete compat gaps I hit going from "Docker SDK against Podman socket works for most calls" to "works for chaos tooling specifically":

1. `ContainerExecStart` with zero flags — Docker accepts via implicit HTTP hijack; Podman rejects with "must provide at least one stream." Switched to `ExecAttach` + drain.
2. Cgroup scope names: `docker-<id>.scope` vs `libpod-<id>.scope`, plus a nested `container/` leaf under systemd that cgroup v2's internal-processes rule forces you to handle.
3. Sidecar `StopSignal` — Podman honors `StopTimeout` and waits 10s for SIGTERM, but `tail -f` as PID 1 ignores SIGTERM, so you eat 10s per cleanup unless you force `SIGKILL`.

Rootless is intentionally unsupported for now (needs slirp4netns/pasta + user-ns cgroup math — separate release).

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
```

---

## 💬 Dev.to / Hashnode

**Title:**

```
Pumba v1.1.0: Native Podman Support, and What "Docker-Compatible API" Actually Means
```

**Tags:** `podman`, `kubernetes`, `chaosengineering`, `go`, `devops`

**Body:**

```
Pumba — a container chaos CLI I've maintained since 2016 — just shipped v1.1.0 with native Podman runtime support alongside Docker and containerd.

I'd expected this to be the quiet release. Podman advertises a Docker-compatible API. The Docker SDK connects to its socket and most calls work unchanged. That part turned out to be true.

What I didn't expect: the 10% where it *doesn't* match are exactly the calls a chaos tool lives on.

## The landmines

### 1. `ContainerExecStart` with empty options

Docker accepts `ExecStartOptions{}` — no AttachStdout, no AttachStderr, no Detach. Podman rejects it outright: *"must provide at least one stream to attach to."* Four callsites in pumba (tc exec, iptables exec, exec-on-container, command-existence check) had to switch from `ContainerExecStart` to `ContainerExecAttach` + drain + `ContainerExecInspect`. About sixty test mocks needed updating for flags Docker didn't require.

### 2. Cgroup path divergence

- Docker: `docker-<id>.scope`
- Podman: `libpod-<id>.scope`
- Podman + systemd: often nests a `libpod-<id>.scope/container/` leaf as libpod's init sub-cgroup
- cgroup v2 forbids processes in internal nodes, so stress-ng sidecars must target the nested leaf when it exists
- Podman's default `cgroupns=private` means `/proc/self/cgroup` inside the target is `0::/` — ancestry hidden

Pumba now reads `/proc/<pid>/cgroup` host-side. Which means pumba must run on the same kernel as the targets. On macOS: inside the `podman machine` VM. Same pattern we already used for containerd-in-Colima.

### 3. Sidecar reap

tc sidecars run `tail -f /dev/null` as PID 1. PID 1 ignores SIGTERM. Podman's DELETE-with-force sends SIGTERM, waits `StopTimeout` (default 10s), then SIGKILLs. Every netem call was paying 10s per cleanup. Fix: `StopSignal: "SIGKILL"` on the sidecar. Immediate reap.

### 4. Cleanup vs. caller cancellation

If pumba itself is SIGTERM'd between `tc` exec and sidecar removal, the cleanup defer never runs — sidecar leaks and the netem qdisc persists on the target's netns. Cleanup now uses `context.WithoutCancel(ctx)` with a 15s budget so the defer actually survives cancellation.

## What's not in the release (honestly)

Rootless Podman support. Detected at client init from `Info.SecurityOptions`; netem/iptables/stress fail fast with guidance (`podman machine set --rootful` or the rootful systemd unit). Doing rootless correctly needs slirp4netns/pasta netns handling and user-namespace cgroup math — that's a release of its own, not a marketing-shaped hack.

## The broader lesson

"Compatible API" is usually harder to integrate against than "completely different API." With a different API you build a fresh mental model and check every call. With a compatible one you assume parity and discover the exceptions empirically.

## Links

- Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
- Repo: https://github.com/alexei-led/pumba

If you're running chaos tests on Podman and hit something I missed — open an issue.
```

---

## 💬 Mastodon (fediverse — #podman, #kubernetes, #chaosengineering)

```
Just shipped Pumba v1.1.0 — native Podman support for container chaos testing.

Short version of what I learned: "Docker-compatible API" is ~90% free and ~10% landmines. ContainerExecStart with empty options works on Docker, rejected by Podman. Cgroup paths diverge (libpod-* vs docker-*). Sidecars take 10s to reap unless you force SIGKILL.

All fixed. All tested.

Rootful only — rootless is honest future work.

🔗 https://github.com/alexei-led/pumba/releases/tag/1.1.0

#podman #kubernetes #chaosengineering #golang #sre #opensource
```

---

## 💬 Bluesky

```
shipped pumba 1.1.0 — native podman support alongside docker and containerd.

the "docker-compatible api" is ~90% free, ~10% landmines. ContainerExecStart with empty options works on docker, rejected by podman. cgroup paths diverge. sidecars take +10s to reap unless you force SIGKILL.

rootful only for now — rootless is real work, not a one-liner.

https://github.com/alexei-led/pumba/releases/tag/1.1.0
```

---

## 💬 CNCF Slack / Kubernetes Slack — #chaos-engineering, #podman

```
Pumba v1.1.0 out today with native Podman support (alongside Docker and containerd).

Short writeup of the three non-obvious compat gaps I hit: `ContainerExecStart` with empty options (Docker accepts, Podman rejects), cgroup scope naming + nested `container/` leaf under systemd, and sidecar reap costing +10s because `tail -f` as PID 1 ignores SIGTERM.

Rootful only for now. Rootless is a separate project (slirp4netns/pasta, user-ns cgroups).

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba
```

---

## 📨 Podman mailing list / GitHub Discussions (containers/podman)

**Subject / Title:**

```
Pumba 1.1 adds native Podman support — some compat-socket notes and one edge case worth flagging
```

**Body:**

```
Hi all — maintainer of pumba (container chaos CLI) here. We shipped native Podman support in v1.1.0 today: https://github.com/alexei-led/pumba/releases/tag/1.1.0

Most of the Docker-compat socket worked out of the box. A handful of gaps required server-specific handling, all resolved on our side. Listing them here in case it helps anyone else building on the compat layer, and because I'd appreciate correction if I've misread any behavior:

1. `ContainerExecStart` with zero stream flags — Docker accepts, Podman rejects with "must provide at least one stream to attach to." We moved to `ContainerExecAttach` + drain + inspect for all exec paths. This seems intentional and fine by me; just worth documenting for SDK consumers.

2. Rootless detection via `Info.SecurityOptions` (substring `"rootless"`). We use this to fail fast on features that need SYS_ADMIN / raw netns. Reliable across the 4.x versions we tested.

3. Cgroup scope leaf: under systemd we sometimes see `libpod-<id>.scope/container/`, sometimes just `libpod-<id>.scope`. cgroup v2's internal-processes rule pushes us to target the nested leaf when it exists.

4. One real issue we worked around: Podman 4.9.x on Ubuntu 24.04 creates `<scope>/container/` during init, migrates PID 1, and rmdirs the leaf — mid-flight from an outside writer's perspective. Races cg-inject (writes PID into `cgroup.procs` from outside the cgroup). ENOENT on write. Matches containers/podman#20910. Podman 5.x (podman machine, FCOS) is stable. Our test sits in `tests/skip_ci/` for now and we plan to add retry-on-ENOENT in our cg-inject binary.

Rootless support is on our roadmap but honestly: we need netns handling via slirp4netns or pasta and user-namespace-aware cgroup math. Rather ship that properly than half-ship.

Happy to take issues / PRs / corrections.
```

---

## 📧 Email pitch — The New Stack / Red Hat Developer / Fedora Magazine

**Subject:**

```
Tip: Pumba 1.1 ships native Podman chaos-testing support (with a Docker-compat post-mortem)
```

**Body (one-pager):**

```
Hi — I'm Alexei Ledenev, maintainer of Pumba, an open-source container chaos CLI I've been running since 2016. v1.1.0 (shipped 2026-04-24) adds native Podman runtime support alongside existing Docker and containerd support.

Why this might be interesting to your readers:

1. Podman is a meaningful share of the container runtime world now — especially in Red Hat / Fedora / RHEL shops and in developer-laptop workflows replacing Docker Desktop. Chaos engineering for Podman has been conspicuously absent from the open-source tool landscape.

2. The "Docker-compatible API" claim breaks in ways specifically relevant to operational tooling: exec semantics, cgroup path naming, cleanup timing. Each is a real issue many projects building on the compat layer will eventually hit. I have a concrete, documented post-mortem of the gaps and fixes.

3. The Podman 4.9.x inject-cgroup race (containers/podman#20910) caught us in a way that's worth a public writeup — it's subtle, reproducible, and the workaround is non-obvious.

I'd be happy to write a guest piece, 1500-2500 words, covering the technical journey and the broader "how to build on Podman's compat layer safely" angle. Alternatively, happy to be interviewed for a roundup.

Release: https://github.com/alexei-led/pumba/releases/tag/1.1.0
Repo: https://github.com/alexei-led/pumba (~4k stars)
Prior writing: https://itnext.io/pumba-v1-0-chaos-testing-beyond-docker-native-containerd-support-finally-06e4c897faaf

Best,
Alexei
```

---

## 📋 Submit to newsletters (do today)

| Newsletter                       | Submit link                     | Cadence                      |
| -------------------------------- | ------------------------------- | ---------------------------- |
| **KubeWeekly**                   | https://kubeweekly.io           | Weekly — submit ASAP         |
| **DevOps Weekly**                | https://devopsweekly.com/submit | Sunday cutoff                |
| **SRE Weekly**                   | https://sreweekly.com/submit    | Thursday cutoff              |
| **Golang Weekly**                | https://golangweekly.com/issues | Submit if HN/Reddit traction |
| **TLDR DevOps**                  | https://tldr.tech/devops        | Daily, submit early          |
| **Container Journal newsletter** | https://containerjournal.com    | Check submission form        |
| **Console.dev**                  | https://console.dev             | Dev tools focus              |

---

## 📌 Posting order recommendation

1. **Hacker News (Show HN)** — morning US time (9-10am ET), weekday. Highest-variance, highest-upside channel. Post first while you're fresh.
2. **Twitter/X thread** — 1-2h after HN goes live, so the article URL is warm.
3. **Reddit — r/podman** — same day. Direct audience, smaller but high-signal.
4. **Reddit — r/kubernetes** — same day or next morning.
5. **LinkedIn personal** — afternoon same day.
6. **Bluesky + Mastodon** — cross-post, same afternoon.
7. **Reddit — r/devops, r/selfhosted, r/sysadmin** — day 2 (don't blast same day, looks spammy).
8. **Lobste.rs** — day 2-3 if HN thread went well (signals it's technical enough).
9. **Newsletters** — submit same day (they're weekly, pick your timing).
10. **Slack channels (CNCF, k8s, podman)** — scattered through week, one per day.
11. **Email pitches (The New Stack, Fedora Mag, Red Hat Dev)** — day 2, after you have HN/Reddit traction to cite.
12. **Dev.to / Hashnode cross-post** — day 3, link back to primary.
13. **Podman GitHub Discussions / mailing list** — day 4-5, once the post has cooled. Different audience, not a promo channel — frame it as a compat-notes contribution.

---

## 🆕 New venues worth trying for 1.1.0

- **r/podman** — obvious fit, didn't apply to 1.0.0.
- **r/selfhosted** — Podman has real mindshare with self-hosters migrating off Docker Desktop.
- **Bluesky** — grown into a real dev audience since 1.0.0. Same tone as Twitter, less noise.
- **Mastodon (#podman, #kubernetes, #chaosengineering)** — hashtag discovery is stronger than Twitter now for Linux/ops content. @hachyderm.io is the usual dev instance.
- **Lobste.rs** — if you can get an invite, technical crowd appreciates the compat-post-mortem angle.
- **containers/podman GitHub Discussions** — direct line to the Podman team. Don't lead with the promo framing; lead with the compat notes.
- **Fedora Magazine pitch** — Podman is Fedora-native, they run community guest posts.
- **Red Hat Developer blog** — same audience, larger reach. Higher bar, longer turnaround.
- **The New Stack** — has covered chaos engineering before. Pitch the compat-layer angle as the story, not the release.
- **Console.dev newsletter** — features dev tools with real substance. Low noise, high signal.
- **Cloud Native Pod (podcast)** — reach out if v1.1 gets traction; 20-minute interview format fits this story.

---

## ✏️ Things to customize before posting

- [ ] Check star count on GitHub — update "(~4k stars)" in the press pitch.
- [ ] Confirm release URL is live: https://github.com/alexei-led/pumba/releases/tag/1.1.0
- [ ] Pick one HN posting slot (8-10 AM ET weekday, avoid Mondays and Fridays).
- [ ] Decide if you'll write a companion article — if yes, delay Dev.to/Hashnode cross-post until it's up and use it as the primary URL everywhere.
- [ ] Proofread once more for anything that reads as AI-generated (dashes, cadence, any "delve" words). Rewrite in your voice where needed.
