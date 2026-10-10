# Changelog

<a id="top"></a>

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[↑ Back to top](#top)

## [Unreleased]

### Added

- Homebrew tap publish via GoReleaser (`homebrew_casks` → [hrodrig/homebrew-kwd](https://github.com/hrodrig/homebrew-kwd)); Release workflow requires `HOMEBREW_TAP_TOKEN`.

[↑ Back to top](#top)

## [0.2.0] - 2026-10-10

### Added

- Daemon loop when `interval > 0` (or `kwd check --interval N` / `KWD_INTERVAL`): serial check → wait → check, notify on overall ready ↔ not-ready transitions (alert and resolution), readiness table on transition only, SIGINT/SIGTERM → exit 0. No `--daemon` flag.
- Daemon HTTP surface when `http.listen` is set (`KWD_HTTP_LISTEN` / `kwd check --listen`): `/healthz` (raw last-tick `ok`/`unhealthy`, 503 before first tick) and `/metrics` (hand-rolled Prometheus `kwd_*` gauges). Fail-fast bind; shared signal cancel + short Shutdown.
- Notify hysteresis: `confirm_alert` / `confirm_ok` (per-resource streaks → overall firing) and optional `repeat_while_firing`. CLI: `--confirm-alert`, `--confirm-ok`, `--repeat-while-firing` (`Flags().Changed` only).
- FreeBSD / OpenBSD port skeletons under `contrib/freebsd/` and `contrib/openbsd/port/` (CLI + `kubectl-kwd`, man pages, sample config); `make port-freebsd-sync`, `port-openbsd-sync`, `dist-freebsd`, `dist-openbsd`.

### Changed

- Notification payloads carry SPEC-full fields (level/color, title, per-resource body, UTC timestamp, `client.id`, cluster) composed into Slack Incoming Webhook plain `{"text":...}`.
- README aligned with the hrodrig CLI family: hero banner, related-tools links, Security/CodeQL/gghstats badges; Install documents BSD port skeletons (official trees still pending) and unpublished Homebrew.
- Terminal demo: Charmbracelet VHS `docs/demo.tape` → `docs/demo.gif` (help, version, sample config, analyze, `check --help`).

[↑ Back to top](#top)

## [0.1.0] - 2026-10-09

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

[Unreleased]: https://github.com/hrodrig/kwd/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hrodrig/kwd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/hrodrig/kwd/releases/tag/v0.1.0
