# CI and test performance

## Measured experiment

Baseline: commit `e1d9d6d`. Candidate changes only CI configuration, test
fixtures and waits; no production Go code, assertions, chaos durations, test
selection or runtime/architecture matrix changed.

Local measurements on 2026-11-04 used an eight-vCPU Linux arm64 Colima VM
(Ubuntu 24.04, kernel 6.8, Bats 1.10). Both Docker runs used the same Pumba
binary and preloaded images, in sequence on an otherwise empty runtime.
Each measurement is one before/after pair, not a confidence interval.

| Workload | Before | After | Result |
| --- | ---: | ---: | --- |
| Full Docker Bats suite, 140 tests, wall time | 631.48s | 476.88s | 24.5% faster |
| Podman exec Bats suite, 8 tests, sum of test times | 75.043s | 3.264s | Same 8 tests passed |
| Podman removal, one disposable fixture | 10.127s | 0.062s | Explicit immediate cleanup |
| Podman removal, two disposable fixtures | 20.244s | 0.216s | Explicit immediate cleanup |

Both full Docker runs and both Podman exec runs passed with **zero skips**.
[Per-test measurements](ci-performance-results.tsv) retain the same test names
and counts for comparison. Full-suite wall times include runner overhead and
therefore differ from sums of Bats test timings.

The Podman experiment used Ubuntu's Podman 4.9.3 in a disposable privileged
container with VFS storage and disabled cgroups. This reproduces fixture removal
and exec behavior, not native Podman cgroup or network coverage. A nested
Podman netem smoke attempt timed out and is not counted as verification.
An initial all-skipped exec attempt was also discarded.

Five warm-image samples compared `podman pull docker.io/library/alpine:latest`
with `podman image exists docker.io/library/alpine:latest`: median
**1.618s versus 0.017s**. Registry latency varies; do not extrapolate an exact
whole-job saving from this microbenchmark.

## Changes retained

- Docker fixture creation polls for running state instead of sleeping one
  second. Create failures and readiness timeouts propagate. Offline tests cover
  immediately ready, delayed, failed-create and timeout cases.
- Disposable Podman fixture removal uses `rm -f --time=0`, including the
  separate dry-run plan helper. Cleanup still targets only the recorded IDs and
  their matching sidecars; protected/unrelated-container regressions remain.
- Podman setup checks for existing images and pulls on a miss. Pull failures
  propagate. CI's initial image pulls remain, so each job still refreshes images.
  Local tests reuse installed images; pull explicitly when testing image updates.
- Four Docker fixture-only stops use `--time=0`. Pumba's graceful-stop,
  restart-duration and cleanup behavior remains under test.
- A restart test polls for a changed start timestamp instead of sleeping five
  seconds. Limit tests inspect final state after synchronous Pumba completion;
  they retain their exact-count assertions without extra idle sleeps.
- Build and Release use the same pinned golangci-lint Action and v2.15.0 as the
  standalone lint workflow, rather than compiling the linter from source.
  Full-project lint stays inside the required `Lint and Test` job and in the
  release build. No new-issues-only filtering was enabled.

The linter change has not been timed in a new hosted run. In the
[inspected PR run](https://github.com/alexei-led/pumba/actions/runs/41692450868),
source installation took about 54s before 19.6s of linting. Prebuilt installation
is recommended by the [linter maintainers](https://golangci-lint.run/docs/welcome/install/local/).
Podman's [4.9 removal contract](https://docs.podman.io/en/v4.9.3/markdown/podman-rm.1.html)
documents the explicit cleanup timeout.

## Reproduce and compare

Use an empty disposable runtime: legacy suites have fixed names, broad selectors
and teardown commands. Do not run them alongside unrelated containers.

On a Linux host with Docker, Bats, its support/assert libraries and the workflow's
images installed:

```bash
make build TARGETARCH="$(dpkg --print-architecture)"
export PATH="$PWD/.bin/github.com/alexei-led:$PATH"
shopt -s extglob
bats --tap --timing --print-output-on-failure tests/!(containerd_*|podman_*).bats
```

On a native rootful Podman host with the binary and images installed:

```bash
bats --tap --timing --print-output-on-failure tests/podman_exec.bats
```

Run the same commands against a pristine baseline and the candidate. Hold the
binary, images, runner and background workload constant; retain TAP output and
compare counts, skips, failures, individual timings and total wall time. Never
treat skipped tests as passing performance evidence.

## Other opportunities: not applied

| Candidate | Evidence and reason to defer |
| --- | --- |
| Start integration alongside unit/reporting jobs | The inspected PR gate took 171s. A small classification prerequisite could overlap that work without deleting checks, but needs hosted queue/cost measurements. Savings overlap with faster lint installation. |
| Balance Docker files across separate runners | Preserves cases and architectures, but adds VM setup and runner demand. Measure the new hosted bottleneck before adding shards. Same-host parallelism is unsafe with current selectors and cleanup. |
| Reduce full-history checkout cost | Checkouts varied from about 2s to 32s. Builds consume Git-derived version metadata, so shallow checkout needs metadata-equivalence checks; no history behavior changed here. |
| Avoid eager Makefile discovery | Three local dry runs averaged about 0.86s for both `make -n test-shell` and `make -n build`. Too small to justify a caching abstraction in this patch. |
| Remove target-side stress-ng installation in Podman fixtures | Current stress tests use a sidecar, but changing these fixtures needs a native cgroup-enabled Podman verification run. |
| Shorten natural-expiry injections | Many Docker network cases deliberately await 10s duration expiry. Preserve expiry-and-revert coverage; do not replace it with cancellation solely for speed. |
| Move tests to release-only | Not needed for the measured gains. Keep the full PR matrix; do not defer runtime regressions until publication. |
| Parallelize release binaries | Four serial builds took about 80s in release 1.5.0, but release latency was already about four minutes. Lower priority than integration execution. |

## Limits and remaining checks

These measurements do not establish an end-to-end hosted CI speedup.
Docker becomes the limiting runtime after the large Podman teardown waits are
removed; per-job savings cannot simply be added to predict PR wall time.

Native Podman's full suite and linux/amd64 execution still need hosted
verification. The existing six-way matrix remains unchanged. Repository branch
protection was read, not modified: `Lint and Test` is required, while integration
statuses are not required there. Removing its lint merely because another
workflow also lints would weaken that gate.
