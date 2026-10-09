# Changelog

<a id="top"></a>

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[↑ Back to top](#top)

## [Unreleased]

### Added

- `kwd check` single-pass readiness check for `deployment` and `statefulset` kinds.
- `kwd analyze` (validate config) and `kwd target` (print resolved API target).
- Slack notification sink (`notifications.sinks[]`, fail-closed `KWD_SLACK_WEBHOOK`).
- `client.id` identity cascade (`KWD_CLIENT_ID` → `client.id` → hostname → IPv4 → MAC → `unknown`).
- `cluster` and `kube.context` config blocks; `--dry-run` (checks + verdict, no notify).
- `internal/exitcode` semantics: `0` success, `1` not-ready/errored, `2` doctor preflight (reserved).

### Fixed

- Readiness is judged against `spec.replicas` (desired) instead of `status.replicas`: a `deployment`/`statefulset` that wants replicas but whose status has observed none — fresh create, `Recreate`-strategy rollout, or scale-up from zero — is reported **not-ready** instead of falsely **ready**. `spec.replicas: 0` remains a deliberate, ready scale-to-zero.

### Security

- Cleared 12 reachable `govulncheck` advisories: bumped `golang.org/x/net` to v0.60.0 and `golang.org/x/text` to v0.42.0 (pulling `golang.org/x/sys` v0.48.0 and `x/term` v0.46.0), and pinned the toolchain to Go 1.27.2 for the patched standard library. `make security` fails again on any new advisory.

[↑ Back to top](#top)
