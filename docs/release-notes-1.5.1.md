## Summary

Pumba 1.5.1 is a maintenance release. CLI flags, result schemas and chaos behavior are unchanged.

## Changes

- PR and merged-commit CI require the complete runtime/architecture matrix. Independent checks overlap, and isolated integration shards preserve every test case.
- Test fixtures avoid unnecessary sleeps, repeated image pulls and graceful-stop waits. Production cleanup and natural-expiry assertions remain intact.
- Test-correctness lint supplements the existing production rules. Nightly/manual validation adds race and advanced runtime tests.
- Release binaries build in parallel OS groups. Images package the verified Linux artifacts instead of recompiling them, and publication requires successful full CI for the exact commit.
- Image payloads are checked against their binary artifacts. Incomplete Docker Hub digest sets no longer publish partial multi-architecture tags.
- Nettools PRs build and validate images without publishing them.

No migration is required. Existing [Docker advisory applicability and daemon-security limitations](https://github.com/alexei-led/pumba/blob/1.5.1/docs/security.md) remain; this release is not a vulnerability-free claim.

[Full changes](https://github.com/alexei-led/pumba/compare/1.5.0...1.5.1)
