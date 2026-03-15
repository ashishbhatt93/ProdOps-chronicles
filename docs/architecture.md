# Architecture

ProdOps Chronicles is a self-hosted DevOps learning platform. This document
describes how the components fit together, how data flows through the system,
and the design decisions behind the key structural choices.

## Overview

```
learner terminal
      │
      ▼
prodops CLI (Go + Cobra)
      │  HTTP + Bearer token
      ▼
Backend API (Go + chi)   ──►  PostgreSQL  (private + public schemas)
      │
      ├──► Runtime Adapter  ──►  Docker Compose (v1.0)
      │                      or  k3s            (v2.0, not yet implemented)
      │
      └──► Module Pods (one per active module)
               │
               ├── /health
               ├── /content/module
               ├── /content/acts/:id
               └── /verify
```

The learner interacts exclusively through the `prodops` CLI. No web frontend
exists. The CLI talks to a local backend via HTTP. The backend manages
progress, orchestrates module pods via the runtime adapter, and delegates
task verification to the pods themselves.

## Configuration

`base_configs.yaml` is the single source of truth for all runtime behaviour.
The backend reads it at startup. No path is hardcoded anywhere in the codebase.

### What lives in base_configs.yaml

| Section | Controls |
|---------|----------|
| `storage` | All filesystem paths. Sub-paths derive from `base_path` automatically. |
| `versions` | Pinned image versions for Postgres, Gitea, Woodpecker. Never `latest`. |
| `runtime` | `compose` (v1.0) or `k3s` (v2.0). Selects the runtime adapter. |
| `difficulty` | `d1` / `d2` / `d3`. Controls hint availability and module visibility. |
| `modules` | Per-module `enabled`, `min_difficulty`, optional port and score override. |
| `ai` | Provider and API key for the optional AI learning agent. |
| `telemetry` | Consent tier (tier1 = private, tier2 = anonymous, tier3 = public). |

### Difficulty levels

| Code | Label | Hints | Effect |
|------|-------|-------|--------|
| `d1` | DevOps Engineer | All 3 | Standard experience. Default. |
| `d2` | Senior DevOps Engineer | First 2 | Reduced hints. Tighter incident windows. |
| `d3` | DevOps Team Lead | First 1 | Minimal hints. Strictest scoring. |

Difficulty is enforced in two places: the backend filters hints before sending
act content to the CLI, and `ModuleService.ListModules` omits any module whose
`min_difficulty` exceeds the player's level.

### Module gating — two layers

A module must pass both layers before a player can enable it:

**Layer 1 — base_configs.yaml**
- `enabled: true` must be set
- The player's difficulty must be >= `min_difficulty`
- Failing here returns `LOCKED` with a message explaining which condition failed

**Layer 2 — database**
- The module must be unlocked for the current run (earned by completing the prerequisite, or force-unlocked with `--force`)
- Failing here returns `LOCKED` with the prerequisite module name

This separation keeps installation-level configuration (what exists) distinct
from progress-level state (what the player has earned).

## Directory Layout

```
prodops-chronicles/
  cmd/
    prodops/        CLI entrypoint (Cobra, 18 commands)
    server/         Backend API entrypoint
    pod/            Module pod entrypoint (one binary, six images)
  internal/
    api/
      handler/      HTTP handlers (player, runs, modules, gameplay, progress, config, sync, internal)
      middleware/   auth, logging, recovery
      router.go
    config/         base_configs.yaml loader — single source of truth
    service/        Business logic (player, run, module, game, verification, progress, sync, yearly_review)
    repository/
      interfaces.go 7 repository interfaces
      postgres/     pgx implementations
    uow/            Unit of Work (pgx transaction wrapper)
    runtime/
      interface.go  RuntimeAdapter interface
      compose/      Docker Compose implementation (v1.0)
      k3s/          k3s stub (v2.0, returns ErrNotImplemented)
    content/        YAML loader, validator, in-memory content types
    pod/            Pod HTTP server, verifier, seeder
    domain/         Shared types and sentinel errors
  migrations/       16 SQL migration pairs (up + down)
  modules/          Module content (YAML acts, endings, community contributions)
    linux-cli/v1/
    git/v1/
    bash-scripting/v1/
    docker/v1/
    docker-compose/v1/
    cicd/v1/
  docker/           Dockerfiles (backend.Dockerfile, pod.Dockerfile)
  scripts/          install.sh
  docs/             install.md, architecture.md
```

## Database

Two PostgreSQL schemas on a single database:

### `private` schema

Accessible only by the `prodops_app` role. Never included in `prodops sync` exports.

| Table | Purpose |
|-------|---------|
| `player_identity` | Single row — name, current run reference |
| `player_profile` | git identity, SSH key path, sync remote, telemetry consent |
| `player_credentials` | API keys (JSONB) — never synced, never logged |

### `public` schema

Readable by the `prodops_sync` role (SELECT only — structurally cannot touch `private`).

| Table | Purpose |
|-------|---------|
| `runs` | One row per chronicle run. Unique partial index enforces one `in_progress` run. |
| `modules` | Static catalog seeded from YAML at pod startup. |
| `module_unlocks` | Which modules a run has earned access to. |
| `module_progress` | Per-run per-module status, tracker state, completed acts/tasks. |
| `decisions_made` | Append-only decision log. Never updated, never deleted. |
| `tracker_snapshots` | Tracker state captured at the moment each decision was made. |
| `task_completions` | Attempt counts, first pass timestamp, lock state, check results. |
| `performance_scores` | Append-only score ledger. Summed to produce performance score. |
| `yearly_review_flags` | Flags raised by decisions. Can be offset by later module completions. |
| `module_completions` | Completion record per attempt — preserves history across replays. |
| `player_setup_meta` | Public-schema copy of consent level and default branch name. |

### Key patterns

**Partial unique index on `runs`**
```sql
CREATE UNIQUE INDEX runs_player_active_uq
  ON public.runs (player_id)
  WHERE status = 'in_progress';
```
Enforces at most one active run per player at the database level, not in application code.

**Append-only ledgers**
`decisions_made` and `performance_scores` are append-only. The application never issues UPDATE or DELETE against them. The full audit trail is always preserved.

**Unit of Work**
Three operations require atomicity. Each uses a `pgx.Tx` wrapped in a `UnitOfWork`:
- `MakeDecision` — UpdateTrackers + RecordDecision + AppendCompletedAct
- `resolveEnding` — CompleteModule + AppendScore + AddFlags + UnlockNextModule + RecordCompletion
- `CompleteRun` — RunRepository.CompleteRun + SetCurrentRun(nil)

All repository methods accept a `repository.Executor` interface satisfied by both `*pgxpool.Pool` and `pgx.Tx`, so they work inside or outside a transaction without changing their signature.

## Module Pods

Each module runs as a Docker container (or k3s pod in v2.0). All six v1.0 modules are built from one Dockerfile with a build argument selecting the content:

```dockerfile
ARG MODULE_ID=linux-cli
COPY modules/${MODULE_ID}/v1/ ./content/
```

A pod's startup sequence:

1. Load and validate all YAML content from `./content/`
2. Connect to the backend with retry (5 attempts, 2s backoff)
3. Version check — if the DB already has this content version, skip seeding
4. Seed: POST to `/api/v1/internal/modules/seed` (bearer-authenticated)
5. Write exercise scaffolding to `/module/state/` (idempotent)
6. Start HTTP server on `:8080`

The pod exposes four routes:

| Route | Purpose |
|-------|---------|
| `GET /health` | Healthcheck for Docker / k3s |
| `GET /content/module` | Full module content |
| `GET /content/acts/:id` | Single act |
| `POST /verify` | Execute one check, return result |

The backend calls `/verify` once per check per task. All checks in a task run even if earlier ones fail — the learner gets full feedback, not just the first failure.

### Verification check types

| Type | How it works |
|------|-------------|
| `exit_code` | Runs a shell command inside the pod, checks the exit code |
| `file_match` | Reads a file from the bind-mounted learner home, checks for a string |
| `regex_output` | Runs a command, matches stdout against a regex pattern |

The learner's home directory is bind-mounted read-only (`/home/${USER}:/home/learner:ro`). Pods never write to learner home.

## YAML Content Structure

```
modules/linux-cli/v1/
  module.yaml              Module metadata and act manifest
  acts/
    01-first-contact.yaml  Act with tasks, checks, hints, and decision
    02a-*.yaml             Branch act (reached via specific decision option)
    ...
  endings/
    endings.yaml           All endings for this module
  community/
    acts/                  Contributor acts (appended after core acts)
```

Each act file contains tasks (with typed checks and tiered hints) and exactly one decision at the end. Each decision option records tracker deltas and an optional yearly review flag. Endings are evaluated as a pure function of the final tracker state — the first ending whose conditions all pass wins.

## Progress Sync

`prodops sync` exports two files to a git-tracked directory (`/opt/prodops/sync/`):

- `progress.json` — structured snapshot (decisions, scores, endings, tracker states)
- `PROGRESS.md` — human-readable GitHub-renderable markdown

The export uses an allowlist pattern for telemetry — only explicitly named fields are included in the anonymous snapshot. The `private` schema is structurally inaccessible to the `prodops_sync` DB role regardless of consent level.

## Runtime Adapter

The runtime adapter interface abstracts all container orchestration:

```go
type RuntimeAdapter interface {
    StartModule(ctx, moduleID) error
    StopModule(ctx, moduleID) error
    ModuleStatus(ctx, moduleID) (PodStatus, error)
    ListRunning(ctx) ([]PodStatus, error)
    WriteModuleDefinition(ctx, moduleID) error
    RemoveModuleDefinition(ctx, moduleID) error
}
```

The Docker Compose adapter (`v1.0`) generates and injects service blocks into `docker-compose.yml`. The k3s adapter (`v2.0`) is a stub returning `ErrNotImplemented` — it will use `k8s.io/client-go` to apply manifests when implemented.

The runtime is selected once at startup from `base_configs.yaml runtime:` and injected as an interface. No `if/else` on runtime type appears anywhere in business logic.

## API

The backend exposes 28 routes under `/api/v1`, all requiring `Authorization: Bearer <token>`. One additional unauthenticated route — `GET /health` — is used by Docker healthchecks and the CLI's `prodops start` polling loop.

A full route reference is in `docs/api-reference.docx`.

## CLI Commands

The `prodops` CLI has 18 commands. Key design choices:

- `prodops verify` and `prodops decision` never require task or act IDs — the backend derives the current position from the player's progress state
- `prodops lesson next` auto-discovers the current in-progress module and fetches the current act
- `POST .../tasks/current/verify` always returns HTTP 200 — a failed verification is a gameplay event, not an HTTP error; execution errors (pod unreachable) return 500

A full command reference is in `docs/cli-reference.docx`.

## Design Decisions

The full rationale for every architectural choice is documented in
`prodops-decisions-v4.docx`. The most significant decisions:

| Decision | Choice | Reason |
|----------|--------|--------|
| Database | PostgreSQL | Scales to SaaS, avoids SQLite→Postgres migration pain |
| Language | Go | Language of the DevOps ecosystem (kubectl, Terraform, Docker are all Go) |
| CI/CD (hands-on) | Gitea + Woodpecker | Lightweight, real pipeline concepts, < 100 MB combined |
| Kubernetes | k3s | CNCF-certified, runs on 512 MB RAM, ships with Traefik |
| Installer | Bash | Zero dependencies, fast to ship; Go binary planned for v2.0 |
| Git location | Host machine | git is a local tool; teaching it inside a pod creates false muscle memory |
| Config source of truth | base_configs.yaml | One file controls all paths, versions, modules, and difficulty |
