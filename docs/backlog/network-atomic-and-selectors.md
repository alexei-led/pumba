# AND-combined selectors (`--match-all`)

Tags: network, safety · Effort: M

## Status

Atomic rollback of partially installed rules shipped in 1.3.1 (#376).

## Why

Each selector (`--source`, `--src-port`, `--target`, `--ingress-port`...) installs its own OR'ed rule. Stacking selectors widens the blast radius instead of narrowing it.

## Approach

- `--match-all` flag first.
- Make AND the default in a major release.

## Open questions

- tc filter AND (multiple matches in one u32 filter) vs separate filters.
- Breaking-change policy for the default flip.
