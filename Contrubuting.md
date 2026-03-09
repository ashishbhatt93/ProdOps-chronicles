# Contributing to ProdOps-chronicles

Thank you for your interest in contributing. ProdOps is a community-driven project — contributions of all kinds are welcome, from bug fixes and documentation to new module content.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Ways to Contribute](#ways-to-contribute)
- [Getting Started](#getting-started)
- [Commit Convention](#commit-convention)
- [Pull Request Process](#pull-request-process)
- [Contributing Module Content](#contributing-module-content)
- [Reporting Bugs](#reporting-bugs)

---

## Code of Conduct

Be respectful. Be constructive. We are all here to learn and build something useful together.

---

## Ways to Contribute

- **Bug reports** — found something broken? Open an issue.
- **Documentation** — typos, unclear steps, missing docs.
- **Module content** — new questions, exercises, or scenarios for existing modules.
- **New modules** — propose a new learning module via an issue before building it.
- **Code** — backend, CLI, install script, infrastructure improvements.
- **Testing** — test on different Linux distributions and report results.

---

## Getting Started

1. Fork the repository
2. Clone your fork

```bash
git clone https://github.com/ashishbhatt93/ProdOps-chronicles.git
cd prodops
```

3. Create a feature branch — never commit directly to `main`

```bash
git checkout -b feat/your-feature-name
```

4. Make your changes
5. Commit following the [commit convention](#commit-convention)
6. Push to your fork and open a Pull Request

---

## Commit Convention

ProdOps uses [Conventional Commits](https://www.conventionalcommits.org/).

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

**Types:**

| Type | When to use |
|------|-------------|
| `feat` | A new feature or module |
| `fix` | A bug fix |
| `docs` | Documentation changes only |
| `chore` | Maintenance, dependencies, config |
| `refactor` | Code change that is not a fix or feature |
| `test` | Adding or updating tests |
| `ci` | CI/CD pipeline changes |

**Examples:**

```
feat(installer): add postgres system user creation with UID check
fix(git-module): correct bind mount path for exercise validation
docs(readme): add quick start instructions
chore(deps): bump postgres image version to 15.4
```

---

## Pull Request Process

1. **One concern per PR** — keep PRs focused. A PR that fixes a bug and adds a feature is two PRs.
2. **Reference the issue** — if your PR closes an issue, add `Closes #<issue-number>` in the PR description.
3. **Update documentation** — if your change affects behaviour, update the relevant docs.
4. **Do not bump version numbers** — maintainers handle versioning.

PRs are reviewed by maintainers. Feedback will be given within a reasonable time. Please be patient.

---

## Contributing Module Content

Module questions and exercises live in `modules/<module-name>/content/` as YAML files. You do not need to write any Go or shell code to contribute content.

**Question format:**

```yaml
# modules/git/content/questions.yaml
- id: git-001
  level: beginner
  type: scenario
  question: |
    Your teammate pushed directly to main without a PR.
    The commit broke the build. What do you do first?
  choices:
    - id: A
      text: git revert the commit and open a PR with the fix
      correct: true
      explanation: |
        Correct. git revert creates a new commit that undoes the change,
        preserving history. This is the safe approach on a shared branch.
    - id: B
      text: git reset --hard HEAD~1 and force push
      correct: false
      explanation: |
        Dangerous on a shared branch. Force pushing rewrites history and
        will cause problems for anyone who already pulled the broken commit.
    - id: C
      text: Delete the branch and start over
      correct: false
      explanation: |
        This would lose all work on the branch, not just the broken commit.
  tip: |
    Pro tip: Use branch protection rules to prevent direct pushes to main.
    In GitHub: Settings → Branches → Add rule → Require pull request before merging.
```

**Rules for content contributions:**
- Questions must be practical — real scenarios, not definitions
- Every wrong answer must have a meaningful explanation of *why* it is wrong
- Every correct answer must include a real-world tip
- Do not include questions that are already in the module (check existing files first)
- Questions must be accurate — when in doubt, test the command or behaviour yourself

---

## Reporting Bugs

Open a GitHub Issue with:

- **What you expected** to happen
- **What actually happened**
- **Steps to reproduce**
- **Your environment**: Linux distro, version, available RAM, ProdOps mode (Beginner/Intermediate/Advanced)
- **Relevant logs** — attach output from `prodops logs` if applicable

---

## Questions?

Open a Discussion on GitHub rather than an Issue for general questions or ideas.
