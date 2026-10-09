# kwd — Kubernetes Workload Watch Dog

<a id="top"></a>

<p align="center">
  <em>Watch your Kubernetes workloads — check readiness in a single pass</em>
</p>

[![Version](https://img.shields.io/badge/version-0.1.0-blue)](https://github.com/hrodrig/kwd/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 1.26.6](https://img.shields.io/badge/Go-1.26.6-00ADD8.svg)](https://go.dev/dl/)
[![CI](https://github.com/hrodrig/kwd/actions/workflows/ci.yml/badge.svg)](https://github.com/hrodrig/kwd/actions/workflows/ci.yml)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/hrodrig/kwd)](https://pkg.go.dev/github.com/hrodrig/kwd)
[![deps.dev](https://img.shields.io/badge/deps.dev-go%20module-blue)](https://deps.dev/go/github.com%2Fhrodrig%2Fkwd)

**Repo:** [github.com/hrodrig/kwd](https://github.com/hrodrig/kwd) · **Spec:** [SPECIFICATIONS.md](SPECIFICATIONS.md) · **Changelog:** [CHANGELOG.md](CHANGELOG.md) · **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)

**The problem:** A rollout finishes, and *someone* has to go look — is the Deployment actually ready? Did the StatefulSet come up, or is a pod crash-looping? Most "health checks" are either ad-hoc `kubectl get` squinting at columns, or a full Prometheus/Grafana stack you don't want to stand up just to answer *"is it up?"*.

**How kwd solves it:** a tiny, single-binary CLI (client-go, **no `kubectl` binary required**) checks the **readiness** of the workloads you declare, in **one pass**, and tells you — exit code `0` if everything is ready, `1` if anything is not. Point it at a context, list your `deployment`/`statefulset` refs, and let cron or CI decide what happens next. Want a ping in Slack when something's down? kwd does that too.

Declarative, out-of-band, and easy to script — the "is this cluster healthy *right now*" answer without a monitoring stack.

<a id="terminal-demo"></a>
**Terminal demo** (recorded with [VHS](https://github.com/charmbracelet/vhs); source `docs/demo.tape`):

![kwd CLI — single-pass readiness check](docs/demo.gif)

## Table of contents

- [How it works](#how-it-works)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Readiness semantics](#readiness-semantics)
- [Exit codes](#exit-codes)
- [Notifications](#notifications)
- [Client identity](#client-identity)
- [Multi-cluster](#multi-cluster)
- [RBAC requirements](#rbac-requirements)
- [Install](#install)
- [Build](#build)
- [Testing](#testing)
- [Docker](#docker)
- [Roadmap](#roadmap)
- [Get involved](#get-involved)
- [License](#license)

---

## How it works

```
config.yml ─► resolve client identity ─► load Kubernetes client ─► single-pass check
                                                                        │
                              ┌─────────────────────────────────────────┤
                              ▼                                         ▼
                        render report ──────────────────────► exit 0 (all ready)
                              │                                         │
                              ▼                                         ▼
                       send notification (Slack)             exit 1 (any not-ready)
```

`kwd` loads a declarative YAML config, resolves which cluster to talk to (`kube.context`), builds a typed **client-go** clientset (no `kubectl` subprocess), and checks every declared resource against its **readiness** rules in a single pass. The report is printed to stdout; the exit code is the machine-readable verdict. Notifications fire only when something is **not ready** (and `dry_run` is false).

**Scope (v0.1.0):** `deployment` and `statefulset` kinds, single-pass (`interval: 0`), Slack notifications. Daemon loop (`interval > 0`), additional kinds (`daemonset`, `service`, `pvc`), `doctor` RBAC preflight, and the `kubectl-kwd` plugin land in later releases — see [Roadmap](#roadmap).

[↑ Back to top](#top)

---

## Quick start

```bash
# See all commands and flags
kwd --help

# Generate an annotated sample config
kwd --print-sample-config > kwd.yml

# One-shot check of a Deployment and a StatefulSet in the default namespace
kwd --config kwd.yml
```

Minimal config (`kwd.yml`):

```yaml
cluster:
  name: staging

resources:
  - deployment.default/web
  - statefulset.default/db
```

Check the **current context** of your kubeconfig, or pick one with `kube.context`:

```yaml
kube:
  context: prod
```

`kwd` uses **client-go** directly — a valid kubeconfig (or in-cluster config) is enough; `kubectl` is never invoked.

[↑ Back to top](#top)

---

## Configuration

`kwd` loads settings in order: **config file** → **environment variables** → **CLI flags** (each layer overrides the previous).

| Source | Path / prefix |
|--------|---------------|
| Config file | `kwd.yml` (via `--config` / `KWD_CONFIG`) |
| Environment | `KWD_*` |
| CLI | `--flag` |

**Sample config:**

```yaml
cluster:
  name: staging
  environment: dev
  description: Shared staging workloads

kube:
  context: staging            # empty = current context

client:
  id: kwd-staging-01          # optional; see Client identity

resources:
  - deployment.default/web
  - statefulset.default/db

interval: 0                   # 0 = single pass (v0.1.0)

notifications:
  sinks:
    - type: slack             # webhook URL comes from KWD_SLACK_WEBHOOK

dry_run: false                # true = check + verdict, no notification
```

Full annotated sample: **`kwd --print-sample-config`** or [`configs/kwd.sample.yml`](configs/kwd.sample.yml).

**Resource refs** use `kind.namespace/name` (same compact shape as [kzero](https://github.com/hrodrig/kzero)): `deployment.default/web`, `statefulset.myns/redis`.

**Environment variables** mirror the config keys under the `KWD_` prefix — e.g. `KWD_CLIENT_ID`, `KWD_KUBE_CONTEXT`, `KWD_DRY_RUN`. Use env for secrets (the Slack webhook) and one-off overrides.

[↑ Back to top](#top)

---

## Readiness semantics

`kwd` judges readiness per kind, deliberately avoiding the "stale or zero-valued status" trap:

| Kind | Ready when |
|------|-----------|
| `deployment` | `readyReplicas == replicas && replicas > 0`, and no `Available=False` condition. `replicas: 0` (deliberate scale-to-zero) is treated as **ready**. |
| `statefulset` | `readyReplicas == replicas && replicas > 0`, and no `Available=False` condition. `replicas: 0` treated as **ready**. |

A resource that cannot be read (API error, RBAC denied) is reported as **`errored`**, not `not-ready` — a missing read permission must never look like "healthy".

[↑ Back to top](#top)

---

## Exit codes

| Code | Meaning |
|------|---------|
| **0** | Success — all declared resources ready |
| **1** | One or more resources **not-ready** or **errored** |
| **2** | `doctor` RBAC preflight failed (reserved for a later release) |

The exit code is the contract for cron and CI: `kwd && echo healthy || echo unhealthy`.

[↑ Back to top](#top)

---

## Notifications

When a check finds a **not-ready** resource (and `dry_run` is false), `kwd` sends one alert per configured sink. **v0.1.0 ships the Slack sink.**

```yaml
notifications:
  sinks:
    - type: slack
```

The webhook URL comes from **`KWD_SLACK_WEBHOOK`** (fail-closed): if the sink is declared but the env var is missing, `kwd` errors on stderr rather than silently skipping the alert. Notification delivery is **best-effort** — a failed send is logged, and the check's exit code still reflects resource health, not delivery.

More sinks (and `--force-notification` for delivery testing) land in later releases.

[↑ Back to top](#top)

---

## Client identity

Every notification carries a **client id** so you can tell *which* `kwd` instance is talking when you run several against different clusters:

1. `KWD_CLIENT_ID` env var
2. `client.id` in config
3. hostname
4. primary IPv4 address
5. MAC address
6. literal `unknown`

[↑ Back to top](#top)

---

## Multi-cluster

`kwd` targets **one cluster per process**, selected by `kube.context` (empty = current context). To watch several clusters, run one `kwd` instance per context — cron or a supervisor orchestrates N invocations. `client.id` + `cluster.name` tag every notification so alerts from different clusters stay distinguishable even when they share a sink.

[↑ Back to top](#top)

---

## RBAC requirements

`kwd` only **reads** the resources it checks. The service account / credential it uses needs, per kind:

| Kind | Required access |
|------|-----------------|
| `deployment` | `get` + `list` on `deployments` and `deployments/status` |
| `statefulset` | `get` + `list` on `statefulsets` and `statefulsets/status` |

The `/status` subresource is **not** covered by a bare `get` on the resource — grant it explicitly or the Status field reads as zero-valued and a healthy workload looks broken (or vice-versa).

[↑ Back to top](#top)

---

## Install

**From source (recommended):**

```bash
go install github.com/hrodrig/kwd@latest
```

This installs the binary to `$GOBIN` (default `$HOME/go/bin`). Ensure `$GOBIN` is on your `PATH`.

**Pre-built binaries:** [Releases](https://github.com/hrodrig/kwd/releases) provide binaries (tar.gz, zip), `.deb`, and `.rpm` packages for Linux, macOS, and Windows (amd64 and arm64). Replace `v0.1.0` and `amd64` with your desired version and arch.

| Platform | Command |
|----------|---------|
| **Debian/Ubuntu** | `wget -q -O /tmp/kwd.deb https://github.com/hrodrig/kwd/releases/download/v0.1.0/kwd_v0.1.0_linux_amd64.deb && sudo dpkg -i /tmp/kwd.deb` |
| **Fedora / RHEL / AlmaLinux / Rocky** | `sudo dnf install https://github.com/hrodrig/kwd/releases/download/v0.1.0/kwd_v0.1.0_linux_amd64.rpm` |
| **Alpine** | `wget -qO- https://github.com/hrodrig/kwd/releases/download/v0.1.0/kwd_v0.1.0_linux_amd64.tar.gz \| tar -xzf - -C /usr/local/bin` |
| **FreeBSD** | port or tarball — see [`contrib/freebsd/`](contrib/freebsd/) |
| **OpenBSD** | tarball — see [`contrib/openbsd/`](contrib/openbsd/) |

[↑ Back to top](#top)

---

## Build

```bash
go build -o kwd ./cmd/kwd
# or use GNU Make (repo root GNUmakefile):
make build
make install
```

**FreeBSD:** `/usr/bin/make` is BSD Make; the repo ships a `Makefile` stub that forwards to `gmake`. Install `devel/gmake` (`pkg install gmake`) and a Go toolchain, then `make build` (or `gmake build`). Linux, macOS, and CI use GNU Make, which reads `GNUmakefile` first.

**Release:** from `main`, `git tag v0.1.0`, `make release` (requires [goreleaser](https://goreleaser.com)). Pre-release checks: `make release-check` (lint, test, cover-check ≥ 80%, security). Local snapshot: `make snapshot` → `dist/`.

[↑ Back to top](#top)

---

## Testing

```bash
make test          # all packages
make cover         # unit tests with coverage.out
make cover-check   # fail if total statement coverage < 80%
```

[↑ Back to top](#top)

---

## Docker

**Build from source** — multi-stage `Dockerfile` (Go 1.26.6 build; **distroless/static-debian13:nonroot** runtime): static binary, non-root, no Alpine OS packages.

```bash
make docker-build
```

Release images are published to `ghcr.io/hrodrig/kwd` via GoReleaser + `Dockerfile.release` (same distroless base). Validate:

```bash
docker run --rm ghcr.io/hrodrig/kwd:latest --help
docker run --rm ghcr.io/hrodrig/kwd:latest version
```

Use in-cluster config, or mount a kubeconfig to check a remote cluster.

[↑ Back to top](#top)

---

## Roadmap

**Current:** **v0.1.0** — single-pass `check` for `deployment` + `statefulset`, Slack sink, `analyze` / `target` helpers, `client.id` + `cluster` identity.

| Version | Scope |
|---------|-------|
| **v0.2.0** | Daemon (`interval > 0`), `--force-notification`, HTTP `healthz`/`metrics` |
| **v0.3.0** | Additional kinds (`daemonset`, `service`, `pvc`), `doctor` RBAC preflight, fine-grained retry |
| **v0.4.0** | `kubectl-kwd` plugin + krew, full packaging (man pages, BSD ports, Homebrew) |

Each version is a vertical slice — usable end to end on its own. Release plans are captured per version in `docs/plan-X.Y.Z.md` at each close.

[↑ Back to top](#top)

---

## Get involved

Found kwd useful? We'd love your help to make it better. You can:

- **Report bugs** or **suggest features** — [open an issue](https://github.com/hrodrig/kwd/issues)
- **Contribute code** — see [CONTRIBUTING.md](CONTRIBUTING.md) for how to submit a pull request
- **Star the repo** — it helps others discover kwd

Thanks for using kwd. Happy watching.

[↑ Back to top](#top)

---

## License

[MIT License](LICENSE). See [LICENSE](LICENSE) for the full text.

[↑ Back to top](#top)
