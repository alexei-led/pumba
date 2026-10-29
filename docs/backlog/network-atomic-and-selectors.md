# Atomic selector install + AND-combined selectors

Tags: network, safety · Effort: S (atomic) / M (AND)

## Why
Context: #362, #363.
- If installing selector N fails, rules 1..N-1 stay installed; pumba returns without cleanup (documented in #363). Correctness bug.
- Each selector (`--source`, `--src-port`, `--target`, `--ingress-port`...) installs its own OR'ed rule. Stacking selectors widens blast radius instead of narrowing it.

## Approach
- Roll back already-installed rules on partial failure (iptables and netem).
- AND semantics: `--match-all` flag first; make AND the default in a major release.

## Open questions
- tc filter AND (multiple matches in one u32 filter) vs separate filters.
- Breaking-change policy for default flip.
