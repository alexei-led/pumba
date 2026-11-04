# CI and test performance

## Hosted measurements

Two successful runs on PR #405 compared the fixture fixes alone with the
parallel, sharded workflow. The source code and six runtime/architecture
combinations were unchanged. These are individual runs, not confidence intervals.

| Build and Test workflow | Wall time | Sum of job durations |
| --- | ---: | ---: |
| [Fixture fixes, unsharded](https://github.com/alexei-led/pumba/actions/runs/41790212618) | 10m08s | 39m20s |
| [Parallel checks and isolated shards](https://github.com/alexei-led/pumba/actions/runs/41792024441) | 4m48s | 41m09s |

Sharding and overlap reduced wall time by 52.6% at a 4.6% increase in summed job
time. That sum excludes separate lint, security and nettools workflows and is
not a billing estimate. Earlier PR runs before fixture optimization took about
18 minutes.

The test-name multisets match exactly in both hosted runs:

| Runtime | Tests per architecture | Skips per architecture |
| --- | ---: | ---: |
| Docker | 140 | 0 |
| containerd | 88 | 0 |
| Podman | 96 | 1 |

The Podman skip is the existing rootless-only scenario on rootful runners.
No case moved to release-only. Every meaningful PR integration case still runs
on linux/amd64 and linux/arm64.

## Local fixture experiment

Baseline: commit `e1d9d6d`. Linux arm64 Colima used eight vCPUs, Ubuntu 24.04,
kernel 6.8 and Bats 1.10. The before/after Docker runs used the same binary and
preloaded images on an otherwise empty runtime.

| Workload | Before | After |
| --- | ---: | ---: |
| Full Docker Bats suite, 140 tests, wall time | 631.48s | 476.88s |
| Podman exec Bats suite, 8 tests, sum of test times | 75.043s | 3.264s |
| Podman removal, one disposable fixture | 10.127s | 0.062s |
| Podman removal, two disposable fixtures | 20.244s | 0.216s |

Both Docker runs and both Podman exec runs passed with zero skips.
[Per-test data](ci-performance-results.tsv) preserve their names and counts.
Wall time includes runner overhead and differs from sums of Bats timings.

The local Podman experiment used Podman 4.9.3 nested with VFS and disabled
cgroups. An initial all-skipped exec attempt and a timed-out nested netem
attempt were discarded. The subsequent hosted baseline verified native Podman
on both architectures.

Five cached-image samples measured median `podman pull` time of 1.618s versus
0.017s for `podman image exists`. Registry latency varies. CI still refreshes
images initially; only repeated per-test pulls were removed.

## Retained changes

- Docker fixture creation polls for running state instead of sleeping one second.
  Offline tests cover each labeled container, delayed readiness, create failures
  and timeouts.
- Disposable Podman cleanup uses `rm -f --time=0`, including the dry-run plan
  helper. Recorded IDs, sidecar ownership and protected-container checks remain.
- Fixture-only Docker stops use immediate termination. Production graceful-stop,
  cancellation and natural-expiry assertions remain.
- Change classification precedes independent unit/lint and integration jobs.
  The required `Lint and Test` check now aggregates them and fails on missing,
  failed, canceled or unexpectedly skipped checks.
- `.github/integration-shards.tsv` balances Docker and Podman files across
  separate runners. The selector fails if any file is omitted, duplicated,
  assigned to the wrong runtime or stale. New integration files must be assigned
  explicitly. containerd retains one shard per architecture.
- Prebuilt golangci-lint v2.15.0 replaces source installation. Existing production
  rules remain. A separate test-correctness config checks assertion correctness
  and parallel-test conventions without imposing a bulk assertion-style rewrite.
- Nettools PR jobs have read-only permissions and build/verify tools without
  publishing. Protected-branch jobs retain registry publishing.
- Full functional CI also runs on merged `master` commits. Release validation
  requires a successful protected-branch run for the exact release SHA.
- Nightly/manual Deep Validation supplements PR checks with full Go race tests
  and the existing advanced runtime suite. Native local acceptance passed its
  39 executable scenarios in 290s. An always-skipped placeholder with no
  assertions was removed; no executable scenario was removed.
- Custom fixture interfaces are polled before network assertions; a running PID
  alone is not sufficient entrypoint readiness. Release-build regression tests run
  in temporary workspaces, and CI checks that offline tests preserve coverage XML.
- Closed-wait cancellation now has a deterministic owned-exec test. Existing
  coverage varied by one line depending on which ready select branch ran;
  coverage thresholds were not relaxed.

## Release experiments

On a 15-logical-CPU macOS arm64 host, separate Go caches were warmed only for
linux/amd64 before each four-target build. Module downloads were shared.
Sequential OS groups took 59s; two parallel groups took 50s. All four binary
pairs were byte-identical. The retained Makefile joins every child and propagates
compiler failures; offline tests cover complete artifacts and failure handling.

Release images now package those Linux artifacts. Local source-image build took
16.75s; first artifact packaging took 13.09s and a cached repeat took 3.66s.
These are local cache-dependent measurements, not hosted release predictions.
Same-version CLI schemas matched, and the packaged binary matched its supplied
artifact byte-for-byte.

Validation and binary compilation run independently; both must pass before
publishing. Release compilation does not restore cross-run Go build caches.
Every binary must report the tagged revision and clean VCS metadata before upload. Image jobs compare their extracted payload with the artifact before
manifest publication. Both GHCR architectures are required. Docker Hub remains
best-effort, but incomplete digest sets leave its tags unchanged.

In release 1.5.0, BuildKit spent 55.4s exporting a maximal GitHub cache versus
22.1s compiling on amd64. Packaging removes the duplicate compilation and this
large SDK cache export. The small certificate/timezone stage uses local BuildKit
cache within each job. SBOM and provenance remain enabled. The packaging
Dockerfile retains the existing distro-update policy for certificate/timezone
packages; Hadolint DL3008 flags their unpinned package versions.

## Discarded or deferred experiments

| Candidate | Decision |
| --- | --- |
| Broad test-linter enablement | The displayed 15 findings were capped; 164 diagnostics included bulk style conventions. Retained focused correctness checks instead. No existing production rule was disabled. |
| Mutable target-container pooling | Rejected. Post-fix Podman exec takes only 3.3s for eight cases; namespace/cgroup/rule reset risks outweigh the remaining setup cost. |
| Folding small CLI permutations now | Deferred. After fixture fixes, these are cheap. Keep case-level diagnostics until an equivalent unit boundary justifies removal. |
| Shortening all injections | Rejected. Natural expiry and cancellation are distinct cleanup contracts. |
| Same-host Bats parallelism | Rejected. Fixed names, broad selectors and cleanup interfere across files. |
| Shallow build checkouts | Deferred. Git-derived version metadata requires equivalence checks; measured gains already meet the current CI target. |
| Lazy Makefile discovery | Deferred. Three dry runs averaged about 0.86s; no cache abstraction was justified. |
| More release compiler fanout | Not adopted. Two OS groups provided a measured gain while bounding concurrent compiler groups. |
| Registry/maximal SDK cache tuning | Superseded by packaging verified binaries; no SDK compilation/cache export is needed in release image jobs. |
| Smaller PR suite | Not adopted. Full pre-merge coverage was preserved. |

## Reproduce and maintain

Use empty disposable runtimes. Legacy suites have fixed names, broad selectors
and teardown commands. Do not run them alongside unrelated containers.

On Linux with the workflow dependencies and images installed:

```bash
make build TARGETARCH="$(dpkg --print-architecture)"
export PATH="$PWD/.bin/github.com/alexei-led:$PATH"
bash .github/scripts/integration-files.bash docker 1 > /tmp/bats-files
files=()
while IFS= read -r file; do files+=("$file"); done < /tmp/bats-files
bats --tap --timing --print-output-on-failure "${files[@]}"
```

Repeat for Docker/Podman shards 1 and 2, and containerd shard 1. Compare identical
test-name multisets, skips, failures, wall time and total job time. Rebalance the
manifest from per-file durations when runner behavior changes. Never count
skipped tests as successful performance evidence.
