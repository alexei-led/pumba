# Security Policy

## Supported Versions

Pumba follows a rolling-release model. Security fixes are applied only to the **latest released minor version** on the `master` branch. Users are strongly encouraged to run a recent release.

| Version        | Supported          |
| -------------- | ------------------ |
| latest release | :white_check_mark: |
| older releases | :x:                |

The current release is tracked in [VERSION](VERSION) and on the [Releases page](https://github.com/alexei-led/pumba/releases).

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

Report vulnerabilities privately using GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability):

1. Go to the repository's **Security** tab
2. Click **Report a vulnerability**
3. Fill out the advisory form with as much detail as possible

If you cannot use GitHub's reporting flow, contact the maintainer at **alexei.led@gmail.com** with the subject line `[pumba security]`. Encrypt sensitive details if needed.

### What to include

- Affected version(s) and environment (Docker / containerd version, OS, arch)
- Steps to reproduce, a proof-of-concept, or a minimal test case
- Observed and expected behavior
- Impact assessment (privilege escalation, container escape, DoS, information disclosure, etc.)
- Any known mitigations or workarounds

### Response timeline

| Step                       | Target                                            |
| -------------------------- | ------------------------------------------------- |
| Acknowledgement of report  | within 7 days                                     |
| Initial triage & severity  | within 14 days                                    |
| Fix or mitigation released | best effort, depending on severity and complexity |

This is a volunteer-maintained project; timelines are best-effort. You will be kept informed of progress.

## Disclosure Policy

- Reports are handled under **coordinated disclosure**
- A CVE will be requested for confirmed vulnerabilities with real-world impact
- A GitHub Security Advisory will be published once a fix is available
- Reporters will be credited in the advisory unless they prefer to remain anonymous

## Scope

### In scope

- The `pumba` binary and source code under this repository
- The official container images published to `ghcr.io/alexei-led/pumba` and `gaiaadm/pumba`
- The helper images `pumba-alpine-nettools` and `pumba-debian-nettools`
- Example Kubernetes and OpenShift manifests under `deploy/`

### Out of scope

- Vulnerabilities in upstream dependencies (Docker Engine, containerd, `tc`/`iptables`, `stress-ng`) — report those to their respective maintainers
- Misuse of Pumba against systems without authorization (Pumba is a chaos testing tool; destructive behavior is the point)
- Issues that require the attacker to already have root on the host or control of the Docker/containerd socket (these grant full control of the runtime regardless of Pumba)
- Example/demo scripts in `examples/` intended for local experimentation

## Hardening Guidance for Users

Pumba interacts directly with the container runtime and requires privileged access. When deploying:

- Run Pumba only in **non-production** or controlled chaos-engineering environments
- Limit access to the Docker/containerd socket to trusted users
- Use `--label` filters to scope chaos to intended targets
- Pin container image tags to immutable digests (`@sha256:...`) in production-adjacent environments
- Review the [deployment manifests](deploy/) before applying to any cluster

## Security Tooling

This repository uses:

- **CodeQL** (`security-extended`, `security-and-quality`) on every PR and weekly
- **golangci-lint** with `gosec` for static analysis
- **GitHub Actions** with least-privilege `permissions:` blocks

Thank you for helping keep Pumba and its users safe.
