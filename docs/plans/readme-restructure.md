# README Restructure & Documentation Overhaul

## Context

The current README.md is 1159 lines with massive duplication (Network Tools Images, IPTables, Advanced scenarios sections appear TWICE). Need to restructure into a concise README with separate docs files.

## Reference Projects

- Toxiproxy: clean README with TOC, separate docs
- k6: beautiful badges, quick example, links to docs
- chaos-mesh: architecture diagram with mermaid

## File Structure

### Task 1: Create docs/README.md (User Guide)

Create `docs/guide.md` — comprehensive user guide extracted from README:

- [x] Container chaos commands (kill, stop, pause, rm, exec, restart)
- [x] Container targeting (names, lists, regex, labels, random)
- [x] Recurring chaos (--interval)
- [x] Dry run mode
- [x] Logging and Slack integration
- [x] TLS configuration

### Task 2: Create docs/network-chaos.md

Move all network chaos content here (deduplicated):

- [x] netem commands (delay, loss, loss-state, loss-gemodel, duplicate, corrupt, rate)
- [x] iptables commands (loss with random/nth modes)
- [x] Network tools images (tc-image, iptables-image, combined nettools)
- [x] Advanced scenarios (asymmetric, combined degradation, microservices)
- [x] Architecture support and building nettools images
- [x] Include mermaid diagram showing how pumba injects tc container into target network namespace

### Task 3: Create docs/stress-testing.md

Move stress testing content here:

- stress command usage
- stress-ng image requirements
- Custom stress images
- Examples

### Task 4: Create docs/deployment.md

Move deployment content here:

- Running as Docker container
- Running on Kubernetes (DaemonSet, labels, nodeSelector)
- OpenShift
- Reference deploy/ directory

### Task 5: Create CONTRIBUTING.md

Move build instructions here:

- Prerequisites (Go 1.26+, Docker)
- Building locally
- Building with Docker
- Running tests (unit, integration, lint)
- Project structure overview
- How to contribute (issues, PRs, code style)

### Task 6: Rewrite README.md

Create a new concise README (~150-200 lines):

- Logo and tagline
- Updated badges (GHCR, correct CI workflow names, Go version)
- One-paragraph description
- Mermaid architecture diagram (how pumba works)
- Quick start (install + first command)
- Feature overview table
- Quick examples (one per category: kill, netem, iptables, stress)
- Links to detailed docs
- Docker images section (GHCR primary, Docker Hub deprecated)
- Links: Contributing, License, Community

### Rules

- Do NOT modify any Go source code
- Do NOT modify mock files
- Do NOT modify CI workflows
- Do NOT modify Makefile
- Update VERSION to current (check git tags)
- Use mermaid diagrams where helpful
- Use GHCR images (ghcr.io/alexei-led/pumba) as primary, Docker Hub as deprecated
- Reference `pumba --help`, `pumba <cmd> --help` for full CLI reference instead of duplicating help text
- Keep examples practical and tested
- No duplicate content across files
- Cross-reference between docs with relative links
