# kwd v0 Specifications

`kwd` is a Go CLI that watches Kubernetes workloads and reports cluster
health. It runs in one of two ways selected by a single knob: a **single-pass
check** (script-friendly `exit 0/1`) or a **daemon loop** (continuous
observation with notifications and an HTTP surface).

## 1. Purpose

`kwd` answers one question: **is the cluster healthy?**

It reads a small YAML file describing which resources to watch, checks each
one's readiness, and reports a verdict. In single-pass mode that verdict is an
exit code (`0` healthy, `1` unhealthy). In daemon mode it is a stream of
observations: state transitions pushed as notifications, plus an HTTP
`/healthz` and `/metrics` surface for liveness and monitoring.

`kwd` is a **watchdog**, not an orchestrator. It never mutates cluster state;
it only observes and reports.

## 2. The `interval` contract

`kwd` has no separate "daemon" vs "one-shot" command. A single `interval`
setting selects the runtime shape, exactly like `pgwd`:

| `interval` | Runtime shape |
|-----------|----------------|
| `0` (default) | Single-pass. Check everything once, emit the verdict, exit `0` or `1`. |
| `> 0` (seconds) | Daemon loop. Serial check → wait(`interval`) → check (never concurrent ticks). Notify and print the readiness table on overall state transitions only. Serve HTTP if configured (Phase 3). Graceful SIGINT/SIGTERM → exit `0`. |

This is the core design rule: **the check logic is identical in both shapes.**
Only the loop and the reporting surface differ. No two code paths, no
dispatched `--daemon` flag.

### Evolution order

`kwd` is designed for phased delivery without breaking this contract:

1. **Plan 1 — single-pass.** `interval: 0`, readiness checks, `exit 0/1`.
2. **Plan 2 — loop.** `interval > 0`, same check in a serial daemon loop.
3. **Plan 3 — observability.** HTTP `/healthz` + `/metrics`, hysteresis
   (`confirm_alert` / `confirm_ok`), `repeat_while_firing`.

Each plan lands on the same contract; nothing is reworked or discarded.

### Run shapes (use cases)

The single knob composes into three concrete operating patterns:

| Pattern | Config | Result |
|---------|--------|--------|
| **On-demand check** | `interval: 0` | One pass, exit `0`/`1`. No notification unless something is unhealthy. |
| **Scheduled report** | `interval: 0` + `--force-notification` | One pass that **always** notifies — e.g. a `4:00` cron/systemd-timer job that pushes a cluster health report even when everything is healthy. |
| **Continuous watchdog** | `interval > 0` | Persistent loop, transition-based notifications, optional HTTP `/healthz` + `/metrics`. |

The **scheduled report** pattern is the `pgwd` "run once, always tell me" model:
run `kwd --config kwd.yaml --force-notification` from cron or a systemd
oneshot+`.timer` pair. Scheduling itself (cron/timer units, runbooks) lives in
the operator repo, not here.

## 3. Scope (v0)

### In scope

- Declarative config from YAML (`--config`, default search path).
- Readiness checks for **`deployment`**, **`statefulset`**, **`daemonset`**,
  **`service`**, and **`pvc`**.
- Read-only helper verbs modeled on `kzero`: **`analyze`**, **`target`**,
  **`doctor`** (RBAC hints via SelfSubjectAccessReview), **`notify test`**.
- `kubectl` plugin packaging: a `kubectl-kwd` binary so `kubectl kwd` works
  (alongside the standalone `kwd` CLI).
- Per-check retry with exponential backoff and jitter (configurable `retry`).
- Single-pass **and** daemon runtime shapes (see §2).
- **`--force-notification`** — notify even when healthy (scheduled reports).
- **`--dry-run`** — run checks and print the verdict without sending
  notifications.
- ANSI **`color`** output (`auto` / `always` / `never`).
- Overall check **`timeout`**.
- Optional **multi-sink** notifications (§8): Slack, generic webhook (with
  `discord`/`teams`/`generic` body presets), Loki, and SMTP email.
- Optional HTTP surface (daemon shape): **`/healthz`** and **`/metrics`**.
- State-transition hysteresis to suppress flapping: **`confirm_alert`** /
  **`confirm_ok`** (consecutive checks before alert/resolution).
- **`repeat_while_firing`** — re-notify every interval while unhealthy, or only
  on transitions.
- Client identity resolution with fallback cascade (§8): env var → config →
  hostname → IP → MAC → literal fallback, embedded in every notification.
- `--show-sample-config` to print a functional example configuration.
- Script-friendly exit contract in single-pass shape: `0` = all healthy,
  `1` = something unhealthy.

### Out of scope

- Cluster mutation of any kind (`scale`, `delete`, `apply`, …).
- **Helm release readiness.** A Helm release is not a read-only Kubernetes
  object; its health is the aggregate of the deployments/statefulsets it
  creates. `kwd` watches those workloads directly — no `helm status` / Helm SDK
  dependency. Deliberately excluded.
- Product- or domain-specific checks (specific databases, message brokers,
  queue depths). Those belong in operator scripts layered on top of `kwd`.
- In-cluster GitOps; `kwd` runs from a bastion/workstation like the rest of
  the `hrodrig/*` family.
- Cron/systemd scheduling, operator runbooks, in-cluster manifests → those live
  in the operator repo, not here.
- **Persistent metrics store / history** (e.g. SQLite). `kwd` emits per-tick
  state only; hysteresis is in-process and `/metrics` reflects the latest tick.
  Historical series ("how long was `X` down this week?") is deferred to a
  future version — a deliberate decision, not an omission.

## 4. CLI contract

`kwd` has one primary verb — **`check`** (also the default when no subcommand
is given) — plus a small set of read-only helpers modeled on `kzero`.

### Subcommands

| Command | Purpose |
|---------|---------|
| `kwd check` | Run the readiness check (single-pass or daemon per `interval`). Also the default: bare `kwd` behaves as `kwd check`. |
| `kwd analyze` | Load and validate the config, then print a normalized plan of what would be checked. Never contacts the cluster. |
| `kwd target` | Print the resolved API target: context, cluster, API server, kubeconfig path. |
| `kwd doctor` | Operator preflight: config, API reachability, refs exist, RBAC hints via `SelfSubjectAccessReview` (see §11). |
| `kwd notify test` | Send a test notification to all configured sinks without running a check. |
| `kwd completion` | Shell completion (cobra built-in). |
| `kwd help` / `kwd version` | Help and version/metadata. |

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Path to the YAML config file | `kwd.yaml` |
| `--kubeconfig` | Path to kubeconfig for `client-go` | `KUBECONFIG` env, then `~/.kube/config` |
| `--interval` | Override `interval` (seconds; `0` = single-pass) | from config |
| `--listen` | Override `http.listen` (daemon only; empty = HTTP off) | from config / `KWD_HTTP_LISTEN` |
| `--confirm-alert` | Override `confirm_alert` (consecutive unhealthy ticks before alert) | from config / `KWD_CONFIRM_ALERT` |
| `--confirm-ok` | Override `confirm_ok` (consecutive ready ticks before resolve) | from config / `KWD_CONFIRM_OK` |
| `--repeat-while-firing` | Override `repeat_while_firing` (re-notify every gap while firing) | from config / `KWD_REPEAT_WHILE_FIRING` |
| `--dry-run` | Run checks and print the verdict, but do not send notifications | `false` |
| `--color` | Colorize output | `auto` (`always` / `never`) |
| `--log-format` | Engine log line format | `text` (`json`) |
| `--force-notification` | Send notifications even when all resources are healthy | `false` |
| `--show-sample-config` | Print a sample YAML and exit | — |
| `--version` | Print version and build metadata | — |

`--show-sample-config`, `--version`, `analyze`, `target`, and `doctor` never
send notifications. `--interval`, `--listen`, `--confirm-alert`, `--confirm-ok`,
and `--repeat-while-firing` take effect only on `check`, and only when the flag
is set (`Flags().Changed`). `http.health_path` and `http.metrics_path` are
YAML-only in this version. Remaining YAML keys may gain CLI overrides later.

Exit codes:

| Code | Meaning |
|------|---------|
| `0` | `check` single-pass: every configured resource is ready. Daemon shape: graceful SIGINT/SIGTERM shutdown. `analyze`/`target`/`doctor`/`notify test` succeeded. |
| `1` | Single-pass: at least one resource is not ready, or a check errored. (Daemon does not map last-tick health to exit `1`.) |
| `2` | `doctor` one or more checks failed (config / API / RBAC / refs). |

`--show-sample-config`, `--version`, `analyze`, `target`, and `doctor` exit `0`
on success. The daemon shape (`interval > 0`) runs until signaled; it does not
use the process exit code as the health channel.

## 5. Configuration schema

```yaml
# cluster metadata: "what is being watched" (kzero-style)
cluster:
  name: "example-cluster"
  environment: "dev"
  description: "Reference profile for kwd"

# Kubernetes target selection (single cluster per process)
kube:
  context: ""            # kubeconfig context; empty = current-context

# optional; empty resolves via cascade (env → config → hostname → IP → MAC → unknown)
client:
  id: ""

# optional; notification sinks (array). Secrets resolve from *_env, never inline.
sinks:
  - type: slack
    webhook_url_env: "KWD_SLACK_WEBHOOK"
  - type: webhook
    url_env: "KWD_DISCORD_WEBHOOK"
    body: discord            # generic | discord | teams
  - type: loki
    url_env: "KWD_LOKI_URL"
    labels:
      job: kwd
  - type: smtp
    host_env: "KWD_SMTP_HOST"
    port_env: "KWD_SMTP_PORT"
    user_env: "KWD_SMTP_USER"
    password_env: "KWD_SMTP_PASSWORD"
    from_env: "KWD_SMTP_FROM"
    to_env: "KWD_SMTP_TO"
    use_tls: true

# optional; HTTP observability surface (daemon shape)
http:
  listen: ""            # e.g. ":8080"; empty = disabled
  health_path: "/healthz"
  metrics_path: "/metrics"

# resources to watch: kind.namespace/name (kzero-style compact refs)
# supported kinds: deployment, statefulset, daemonset, service, pvc
resources:
  - deployment.app/frontend
  - statefulset.data/job-queue
  - daemonset.monitoring/fluent-bit
  - service.app/my-api
  - pvc.data/postgresql-0

# per-resource check retry: exponential backoff + full jitter (see §7)
retry:
  attempts: 3
  initial_backoff: 1s
  max_backoff: 8s

# overall check timeout (duration string; e.g. "2m"). A resource not Ready
# within its retries + this ceiling is reported Error.
timeout: 2m

# output color: auto (default), always, never
color: auto

# engine log line format: text (default) | json
log_format: text

# dry-run: print the verdict but do not send notifications
dry_run: false

# check loop (seconds). 0 = single-pass; > 0 = daemon loop.
interval: 0

# hysteresis: consecutive checks before alert/resolution (default 1 each)
confirm_alert: 1
confirm_ok: 1

# re-notify every interval while unhealthy (default false = transitions only)
repeat_while_firing: false

# verbose logging (default true)
verbose: true
```

### Validation rules

- `resources` must be non-empty.
- `cluster.name`, `cluster.environment`, and `cluster.description` are optional
  metadata strings (kzero-style). `cluster.name` is recommended — it
  identifies "what is being watched" in notifications.
- `kube.context` is optional (empty = current-context). A single `kwd` process
  watches exactly one cluster; multi-cluster is achieved by running N
  instances, each with its own `--config`/`kube.context`.
- Every resource must be a valid `kind.namespace/name` reference. `kind` is one
  of `deployment`, `statefulset`, `daemonset`, `service`, `pvc`. `namespace` and
  `name` are non-empty and match `[a-z0-9-]` / `[a-zA-Z0-9._-]` respectively
  (kzero convention).
- `sinks` is optional. When present, each sink must have a valid `type`.
- Each `type` requires its own secrets via `*_env` (fail-closed): a referenced
  env var that is missing or empty is a config error — no inline credentials.
- `sinks[].type` is one of `slack`, `webhook`, `loki`, `smtp`.
- `webhook.body` is one of `generic`, `discord`, `teams` (default `generic`).
- `interval` must be `>= 0`.
- `retry.attempts` must be `>= 1` (default `3`).
- `retry.initial_backoff` and `retry.max_backoff` must be valid positive
  durations, with `initial_backoff <= max_backoff` (defaults `1s` / `8s`).
- `timeout` must be a valid positive duration string (default `2m`).
- `color` must be one of `auto`, `always`, `never` (default `auto`).
- `log_format` must be one of `text`, `json` (default `text`).
- `dry_run` is a boolean (default `false`); when `true`, notifications are
  suppressed but checks and the verdict run normally.
- `confirm_alert` and `confirm_ok` must be `>= 1`.
- If `http.listen` is set, `http.health_path` and `http.metrics_path` default as
  shown and may be non-empty.
- `http.*` has no effect in single-pass shape (`interval: 0`).

## 6. Readiness semantics

| Type | Ready when |
|------|-----------|
| `deployment` | `readyReplicas == replicas` and `> 0`, and no `Available=False` condition. `spec.replicas == 0` counts as `Ready` (scale-to-zero is a valid resting state). |
| `statefulset` | `readyReplicas == replicas` and `> 0`. `spec.replicas == 0` counts as `Ready`. |
| `daemonset` | `status.desiredNumberScheduled == status.numberReady` and `> 0` (all scheduled pods are ready on their nodes). |
| `service` | the service has at least one endpoint with a non-empty `addresses` list. |
| `pvc` | `status.phase == "Bound"`. `Pending` (unbound) or `Lost` is `NotReady`. |

A resource that errors (unreachable, parse failure, RBAC denial) is reported
as an error and contributes to a non-zero verdict.

## 7. Retry, timeout, and output

### Retry

Each resource check retries on failure with exponential backoff and full
jitter, configurable via the `retry` block (pgwd-style, explicit bounds):

- attempts: `retry.attempts` (default `3`)
- initial backoff: `retry.initial_backoff` (default `1s`)
- max backoff: `retry.max_backoff` (default `8s`)
- backoff factor: `2.0`, full-jitter; each delay is capped at `max_backoff`.
- retries apply only to transient errors (API timeout / conflict / 429 / 503);
  a definitive error (e.g. RBAC denial, invalid reference) fails immediately.

### Timeout

`timeout` (default `2m`) is the ceiling for a single check pass. A resource
that has not reached `Ready` within its retries and this ceiling is reported
`Error` (not `NotReady`) and makes the run unhealthy.

### Output color

`color` controls ANSI coloring of the verdict and per-resource status lines:
`auto` (color only when stdout is a TTY), `always`, `never`. It does not affect
notifications or Loki payloads, which are always plain text.

## 8. State and notifications

### Verdict state

`kwd` tracks per-resource state across ticks in the daemon shape. A resource
is `Ready`, `NotReady`, or `Error`. The overall cluster verdict is `healthy`
only when every resource is `Ready`.

### Hysteresis

`confirm_alert` (`confirm_ok`) is the number of consecutive unhealthy (healthy)
verdicts required before an alert (resolution) is emitted. Default `1` means no
hysteresis — every transition notifies. This suppresses flapping on
borderline resources.

### Client identity

Every notification identifies its origin along two orthogonal axes:

- **`client.id`** — *who* is watching (the bastion/host running `kwd`).
- **`cluster`** — *what* is being watched (`name`, `environment`,
  `description`).

The resolved `client.id` comes from a cascade, so a notification is never
anonymous:

1. `KWD_CLIENT_ID` environment variable (highest precedence);
2. `client.id` in the YAML config;
3. hostname;
4. primary interface IPv4;
5. MAC address;
6. literal fallback `unknown`.

The resolved `client.id` is embedded in every notification payload (Slack
attachment field, webhook JSON, Loki label, SMTP subject) and in engine log
lines, exactly like `kzero` propagates `client_id` into its metadata.
`cluster.name` / `cluster.environment` ride alongside it so each alert names
both the watcher and the target.

### Sinks

Notifications are delivered through a list of **sinks** — a declarative array
of destinations in the config. Each sink has a `type` and resolves its
credentials from environment variables (`*_env`), never inline. Supported
types:

| Sink `type` | Payload / notes |
|------------|-----------------|
| `slack` | Slack Incoming Webhook, plain-text message. |
| `webhook` | Generic JSON POST; `body` preset `generic` (default), `discord`, or `teams`. |
| `loki` | Pushes one log line per notification with `labels`. |
| `smtp` | Email via SMTP (`host`, `port`, auth, `from`, `to`, `use_tls`). |

### Fan-out

`kwd` delivers each notification to **all** configured sinks. Delivery
continues if any single sink fails; failures are aggregated and reported. If an
env var referenced by a sink is missing or empty at startup, configuration
fails closed with an error.

### When notifications fire

A notification is emitted when `--force-notification` is set **or** on a state
transition (unhealthy → or → healthy, per hysteresis). Each payload carries a
`color`/level (green healthy, red unhealthy), a title, a text body listing each
resource and its status, a UTC timestamp, plus resolved `client.id` and cluster
metadata. The Slack Incoming Webhook sink encodes those fields into the plain
JSON `text` field (`{"text":...}`); richer attachment JSON is optional polish.
In the daemon shape, the readiness table is printed on the same transition
moments (not every tick). When `repeat_while_firing` is `true`, the unhealthy
notification is re-sent every `interval` while the bad state persists
(Phase 3).

## 9. HTTP surface (daemon shape)

When `http.listen` is set and `interval > 0`, `kwd` serves:

- **`/healthz`** (or `http.health_path`) — `text/plain` body `ok` or `unhealthy`.
  `200` when the **raw** last completed tick reported every resource Ready;
  `503` before the first completed tick or when any resource is not ready /
  errored. Reflects the last completed pass immediately (never waits on an
  in-flight check). Healthz does **not** use post-hysteresis firing state —
  hysteresis gates notifications only.
- **`/metrics`** (or `http.metrics_path`) — Prometheus text exposition
  (`kwd_resource_ready{kind,namespace,name}`, `kwd_check_latency_seconds`,
  `kwd_last_tick_timestamp_seconds`, `kwd_healthy`). Always `200`; before the
  first tick gauges are zeroed (no per-resource samples yet).

Bind failure exits non-zero before the daemon loop starts. SIGINT/SIGTERM
cancels the shared process context: the HTTP server shuts down briefly, then
Daemon returns nil → process exit `0`. Prefer binding to `127.0.0.1` — the
surface is unauthenticated.

The HTTP surface has no effect in single-pass shape.

## 10. Packaging and release artifacts

A `kwd` release ships the following artifacts (same surface as the `hrodrig/*`
family — `kzero` / `groot` / `pgwd`):

| Artifact | Location | Notes |
|----------|----------|-------|
| Standalone binary | GoReleaser `dist/` | `kwd` for linux/darwin + BSD, `arm64`/`amd64`. |
| kubectl plugin | GoReleaser | `kubectl-kwd` binary; krew manifest under `contrib/krew/`. |
| Man pages | `contrib/man/man1/` | `kwd.1` **and** `kubectl-kwd.1`; `.TH` version must match `VERSION`. |
| System package | `contrib/deb/` | `.deb` (with prerm/postrm hooks). |
| BSD ports | `contrib/freebsd/` + `contrib/openbsd/` | synced via `make port-*-sync`. |
| CLI demo | `docs/demo.tape` | vhs source; renders `docs/demo.gif`. |
| Config example | `configs/kwd.sample.yml` | printed by `--show-sample-config`. |

Versioning is semver without a leading `v` in `VERSION`. Releases follow the
family convention: PR `develop` → `main`, annotated tag `v<semver>` on `main`,
GoReleaser artifacts, BSD ports synced. Development process details (version
bump table, `make release-check` gates, changelog) live in `AGENTS.md`, not
here.

## 11. RBAC requirements

`kwd` is strictly read-only. The `kwd doctor` command verifies the active
principal's permissions at runtime via Kubernetes
**`SelfSubjectAccessReview`** — it does not require (or read) a ClusterRole
document; it asks the API server "can *I* do this?".

For reference, the minimal read-only surface per supported kind is:

| Kind | Requires |
|------|----------|
| `deployment` | `get`, `list` on `apps/deployments` and `apps/deployments/status` |
| `statefulset` | `get`, `list` on `apps/statefulsets` and `apps/statefulsets/status` |
| `daemonset` | `get`, `list` on `apps/daemonsets` and `apps/daemonsets/status`, plus `list` on `pods` |
| `service` | `get`, `list` on `services`, plus `list` on `endpoints` |
| `pvc` | `get`, `list` on `persistentvolumeclaims` |

Readiness status fields live on `/status` subresources, which are **not**
covered by a bare `get` on the resource — the operator's `ClusterRole` must
grant them explicitly. `kwd doctor` flags missing permissions before a check
fails silently at 4:00 am.
