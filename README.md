# ProdOps Chronicles

> A self-hosted, hands-on DevOps learning platform. Learn by doing — not by reading.

ProdOps Chronicles is an open-source platform that teaches practical DevOps skills through real, task-based exercises running on your own machine. Every module gives you something to build, configure, or break — and then validates your work via the `prodops` CLI.

---

## What is ProdOps Chronicles?

Most DevOps learning resources teach you *about* tools. ProdOps Chronicles makes you *use* them.

You write real scripts. You build real pipelines. You fix real issues. Then `prodops verify` checks your work — not a multiple choice quiz.

The platform itself is built using the same tools it teaches: Git, Docker, Docker Compose, Gitea, Woodpecker CI, Jenkins, and SonarQube. Reading the codebase is part of the learning.

---

## Status

> 🚧 **Active Development — v0.1** — Repo setup and install script in progress.
> See the [milestones](#milestones) section for the full roadmap.

---

## v1.0 Modules

| Module | Key Tools | Practical Exercise |
|--------|-----------|--------------------|
| Git | git, bash | Branch, commit, PR, rebase, resolve merge conflicts |
| Linux CLI | bash, coreutils, grep, awk, sed | Filesystem navigation, permissions, pipes, scripts |
| Docker | docker, dockerfile, compose | Write Dockerfile, build image, push to registry |
| Bash Scripting | bash, cron, functions | Write scripts with error handling, loops, cron jobs |
| CI/CD | Gitea, Woodpecker CI | Configure pipeline, trigger build, push image |

> Intermediate and Advanced modules (Terraform, Go, DevSecOps, Kubernetes, AWS, and more) are planned for v2.0.

---

## How It Works

```
┌─────────────────────────────────────────────┐
│              your machine                   │
│                                             │
│  prodops CLI (Go + Cobra)                   │
│       │                                     │
│       ▼                                     │
│  Backend API (Go) ──► PostgreSQL            │
│       │                                     │
│       ▼                                     │
│  Docker Compose                             │
│   ├── prodops-git                           │
│   ├── prodops-linux                         │
│   ├── prodops-docker                        │
│   ├── prodops-bash                          │
│   └── prodops-cicd (Gitea + Woodpecker)     │
└─────────────────────────────────────────────┘
```

Each module runs as an isolated Docker container. Module availability is controlled by `values.yaml`. Progress and unlock state are tracked in PostgreSQL.

See [docs/architecture.md](docs/architecture.md) for full details.

---

## Requirements

- **OS:** Linux (Ubuntu 20.04+ recommended)
- **RAM:** 4GB minimum, 8GB recommended
- **Disk:** 10GB free space
- **Arch:** x86_64

> The install script handles everything else.

---

## Quick Start

> Full installation guide: [docs/install.md](docs/install.md)

```bash
# Clone the repo
git clone https://github.com/ashishbhatt93/ProdOps-chronicles.git
cd prodops-chronicles

# Run the install script
chmod +x scripts/install.sh
./scripts/install.sh

# Start ProdOps
prodops start

# List available modules
prodops module list

# Enable your first module
prodops module enable git

# Start a lesson
prodops lesson next

# Do the task, then verify your work
prodops verify

# Check your progress
prodops progress
```

---

## CLI Reference

| Command | Description |
|---------|-------------|
| `prodops start` | Start the ProdOps stack |
| `prodops stop` | Stop the stack |
| `prodops status` | Show running containers and health |
| `prodops module list` | List all modules and their unlock status |
| `prodops module enable <name>` | Enable a module |
| `prodops lesson next` | Get the next lesson |
| `prodops verify` | Verify your completed task |
| `prodops progress` | Show scores, streaks, and unlocks |
| `prodops sync` | Sync progress to your git remote |
| `prodops unlock <name> --force` | Force-unlock a module |

---

## Milestones

| Branch | Milestone |
|--------|-----------|
| `dev/v0.1` | ✅ Repo setup + install script + host config *(current)* |
| `dev/v0.2` | PostgreSQL container + Go backend skeleton |
| `dev/v0.3` | prodops CLI skeleton + core commands |
| `dev/v0.4` | Git module |
| `dev/v0.5` | Linux CLI + Docker modules |
| `dev/v0.6` | Bash Scripting module |
| `dev/v0.7` | CI/CD module (Gitea + Woodpecker) |
| `dev/v0.8` | Progress sync + unlock system |
| `dev/v0.9` | Testing, bugfixes, docs |
| `release/v1.0` | Stable release |

---

## Branching Strategy

```
main                  ← stable releases only, tagged (v1.0, v2.0)
  └── release/v1.0   ← feature freeze, bugfixes only
        └── dev/v0.x ← active development (current: dev/v0.1)
              └── feat/*  ← individual features, merged via PR
```

---

## Contributing

Contributions are welcome — especially new module questions and exercises.

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:
- Adding questions to existing modules (YAML only, no code required)
- Reporting bugs
- Proposing new modules
- Code contributions

---

## License

MIT — see [LICENSE](LICENSE).
