# Vulnerability gate and applicability

## CI contract

The Build and Test workflow runs govulncheck **v1.9.0**, source mode,
`-json -scan=symbol ./...`. This preserves the existing symbol-level gate;
module/package inventory is reported separately, not treated as symbol calls.
The gate uses only Bash and jq, both provided by the runner.

[The gate](../.github/scripts/govulncheck.bash) records the raw JSON stream in
`govulncheck.json` and prints the scanner's actual exit status. The workflow
logs its decision in `govulncheck.txt`; `pipefail` propagates both gate and
logging failures. Tool installation failure also fails the step.

In this pinned version, **JSON mode exits 0 even when findings exist**. Status 3
means findings only in text mode, and is rejected as inconsistent in JSON mode.
See upstream [exit definitions](https://github.com/golang/vuln/blob/v1.9.0/internal/scan/errors.go),
[handler selection and completion](https://github.com/golang/vuln/blob/v1.9.0/internal/scan/run.go),
and [JSON handler](https://github.com/golang/vuln/blob/v1.9.0/internal/govulncheck/handler.go).
The gate requires a successful process, valid complete JSON serialization, the
expected config, an SBOM with roots/modules, and consistent finding references.
The protocol has no end-of-scan marker; process success is essential. Empty,
malformed or truncated JSON, unexpected status, and scanner errors/panics fail,
even after a pre-approved finding was emitted.

The [v1.0.0 protocol](https://github.com/golang/vuln/blob/v1.9.0/internal/govulncheck/govulncheck.go)
classifies findings by their **first trace frame**: `function` means symbol,
`package` without function means package-only, neither means module-only.
A vulnerability may emit multiple findings at progressively deeper levels;
the report shows each advisory/module/version at its deepest observed level.
Any unaccepted symbol finding fails the gate. Unknown module/package-only
inventory remains visible, without adding an exception or changing scan level.

Regression tests run without network or container runtimes:

```sh
bash .github/scripts/test_govulncheck.bash
```

## Existing two-advisory acceptance

The [1.5.0 plan, §6](plans/completed/20261029-1.5.0-agent-ready.md#6-delivery-pr-sequence)
accepts only **GO-2026-4887** and **GO-2026-4883**. The gate restricts that
acceptance to `github.com/docker/docker@v28.5.2+incompatible`, the audited
legacy dependency. A different advisory, module or version is not accepted.
A gate decision accepting these findings is **not a vulnerability-free scan**.

Applicability was checked on master
`c58d83a64b5cee87a788b7df39da36766f111893` on 2026-11-01, with Go 1.27.3,
govulncheck v1.9.0, and the public database updated 2026-10-27. Host source,
Linux amd64 release-tagged source, and Linux amd64 release-style binary scans
reported the two symbol advisories (text scans exited **3**, not 0).

Both [GO-2026-4887](https://vuln.go.dev/ID/GO-2026-4887.json) and
[GO-2026-4883](https://vuln.go.dev/ID/GO-2026-4883.json) lack package/symbol
restrictions for the legacy module. This broad metadata causes client SDK
symbols to be reported, including real application calls, **not just init
chains**. The affected daemon implementations are absent from the audited
Linux executable's dependency graph and symbols:

| Advisory | Affected implementation absent from Pumba | Applicability and upstream fix |
| --- | --- | --- |
| [GO-2026-4887 / CVE-2026-34040](https://github.com/moby/moby/security/advisories/GHSA-x744-4wpc-v9h2) | `github.com/docker/docker/pkg/authorization`: `Ctx.AuthZRequest`, `drainBody` | Docker Engine AuthZ plugins relying on request-body inspection can receive oversized requests without bodies. Fixed in Engine 29.3.1 / `github.com/moby/moby/v2` 2.0.0-beta.8. |
| [GO-2026-4883 / CVE-2026-33997](https://github.com/moby/moby/security/advisories/GHSA-pxq6-2prw-chj9) | `github.com/docker/docker/plugin`: `validatePrivileges`, `isEqual` (upstream now `daemon/pkg/plugin`) | Malicious Docker plugins can bypass the first approved-privilege comparison. Fixed in Engine 29.3.1 / `github.com/moby/moby/v2` 2.0.0-beta.8. |

The audited executable includes Docker `api`, `api/types` subpackages, `client`
and `pkg/stdcopy`, not `daemon`, `plugin`, `pkg/authorization`, `daemon/pkg/plugin`,
`pkg/plugins`, `pkg/chrootarchive`, or `api/server`. Retained SDK methods such as
`PluginInstall` and `CopyToContainer` are not proof of application invocation or
inclusion of vulnerable daemon code. No Pumba plugin-install/archive-copy
application path was found in the audit.

Upstream Engine fixes exist; the Go database still lists **no fixed version for
the legacy `github.com/docker/docker` module** for these advisories. Migrating
to the split Moby client module is a separate, deferred change, not part of this
CI correction.

## Inventory is not an additional exception

The same audit observed three **module-only** findings. These are newly named
inventory evidence, not proven newly introduced vulnerabilities, and are not
added to the symbol acceptance list:

| Advisory | Daemon implementation named by Go database (absent from Pumba) |
| --- | --- |
| [GO-2026-5746 / CVE-2026-41567](https://github.com/moby/moby/security/advisories/GHSA-x86f-5xw2-fm2r) | `daemon.Daemon.containerExtractToDir`: compressed archive extraction via a trojan decompressor in a malicious container |
| [GO-2026-5668 / CVE-2026-41568](https://github.com/moby/moby/security/advisories/GHSA-vp62-88p7-qqf5) | `daemon.Daemon.createIfNotExists`, `openContainerFS`: symlink races in volume-mount setup for archive/copy APIs |
| [GO-2026-5617 / CVE-2026-42306](https://github.com/moby/moby/security/advisories/GHSA-rg2x-37c3-w2rh) | `daemon.Daemon.openContainerFS`: symlink races redirecting volume bind mounts for archive/copy APIs |

Their upstream fixes are Engine 29.5.1 / Moby v2 2.0.0-beta.14, with no fixed
legacy module version listed. Any future symbol finding for these IDs fails.

External daemon versions, AuthZ/plugin settings and host policy remain unknown.
This source gate does not scan deployed daemons, helper images or final release
artifacts. Re-scan the final merged release candidate and shipped artifacts;
neither the audit nor acceptance of two findings constitutes release approval.
