# 1.5.0 publication verification

Release: [1.5.0](https://github.com/alexei-led/pumba/releases/tag/1.5.0).
Tag commit: `e02f41601dcc9824203d9ac4a1b4aab7a49e3446`.
Owning [Release workflow](https://github.com/alexei-led/pumba/actions/runs/41627580568) completed successfully, including both native image builds, both registries and manifest publication.

## Binaries

All four downloaded assets match the workflow's `pumba-binaries` artifact byte-for-byte and their GitHub API SHA-256 digests:

| Asset | SHA-256 |
|---|---|
| `pumba_darwin_amd64` | `29f1f9c920485a67261f20203037b529338062c9b7a94156bdbee5e05a3505aa` |
| `pumba_darwin_arm64` | `3a28233596f94bb6480f23561874db9cf3b09e9647393b9025390d3195c72de2` |
| `pumba_linux_amd64` | `628c537ae629124eb1d8f4943a86984159a21e086f77e5ee9390da69ab85ea54` |
| `pumba_linux_arm64` | `42076c31364a5d1041a129c784eac7a86e0f6a61d8794be30e7398960aa9a86e` |

Embedded revision and version match the tag; CGO is disabled. The macOS arm64 asset reports 1.5.0 and emits the same valid schema in text/NDJSON modes without a runtime socket. Published release text matches the committed notes.

All four actual assets were scanned with govulncheck v1.9.0. JSON scanner status was 0 **with findings**: only the two previously accepted Docker SDK symbol tuples, GO-2026-4883 and GO-2026-4887 at v28.5.2+incompatible. This is not a vulnerability-free result; see [applicability](security.md).

### Build-metadata warning

All four assets record `vcs.modified=true`. The workflow uploads coverage before building; the exact Codecov action revision `303a32d7a59b442fa8d48b6a1cc6825c09c847a5` defaults to downloading its uploader and checksum files into the working directory. The build log records `./codecov`, and those files are not ignored. This explains a dirty checkout without establishing a pristine-source build from buildinfo alone. No tracked source change was identified.

The follow-up workflow sets Codecov's supported `cleanup: true` option, which uses and removes a temporary download directory for future releases. The published 1.5.0 tag and assets are not replaced.

## Images

Both registries publish Linux amd64 and arm64, with matching runnable child manifests:

- `ghcr.io/alexei-led/pumba:1.5.0`: `sha256:5f1fddf9d6e0d85f0a0079a0b9a1989240f164894857139b02054c269d55a328`
- `docker.io/gaiaadm/pumba:1.5.0`: `sha256:4a4c1dba46610c3d26575d0b72d8bfb5ad0b07f8591c88a7c516d18f2426f490`

Each registry's `latest` matches its 1.5.0 index at verification time. Both platforms have SLSA provenance bound to this repository, tag commit and workflow run, plus nonempty SPDX-2.3 SBOMs. These are BuildKit attestations, not a claim of independent signature verification.

The immutable GHCR image passed an arm64 version/schema smoke test in the isolated acceptance VM. The existing default VM and Docker context were not changed.
