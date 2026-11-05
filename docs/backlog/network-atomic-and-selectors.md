# AND-combined selectors (`--match-all`)

Tags: network, safety · Effort: M

## Status

Atomic rollback of partially installed rules shipped in 1.3.1 (#376).

## Why

Each selector (`--source`, `--src-port`, `--target`, `--ingress-port`...) installs its own OR'ed rule. Stacking selectors widens the blast radius instead of narrowing it.

## Approach

- [x] Opt-in `--match-all` on netem and iptables parent commands; existing OR default is unchanged.
- Make AND the default in a major release.

The 1.6.0 implementation bounds selector expansion to 256 combinations and each rendered script to 64 KiB. Larger requests fail before injection; IPv6/protocol expansion remains deferred.

## Open questions

- tc filter AND (multiple matches in one u32 filter) vs separate filters.
- Breaking-change policy for the default flip.
