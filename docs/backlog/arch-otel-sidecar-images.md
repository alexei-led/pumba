# OpenTelemetry spans and sidecar image consolidation

Tags: arch · Effort: S–M

## Approach
- OTel span per fault so chaos windows correlate with traces in Grafana/Jaeger. Verify whether any OTel dependency is direct or only transitive before starting.
- One nettools image (tc, iptables, ip, DNS tooling) version-pinned to the pumba release.

## Open questions
- Exporter config surface (env vars only?).
