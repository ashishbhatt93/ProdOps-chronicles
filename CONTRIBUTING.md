# Contributing to ProdOps Chronicles

## Adding module questions

Community contributions are welcome in the form of new acts (YAML files).

1. Fork the repo
2. Create a feature branch: `feat/module-<name>-act-<description>`
3. Add your act YAML to `modules/<module>/v1/community/acts/NN-your-act.yaml`
4. Follow the schema in `modules/linux-cli/v1/acts/01-the-missing-log.yaml`
5. Open a Pull Request — a maintainer will review for accuracy and quality

Acts merged to `main` trigger a CI rebuild of the relevant module image.

## Commit style

All commits must use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add act for grep -r exercise
fix: correct expected_exit in file_match check
docs: update contributing guide
chore: bump woodpecker to 2.4
```

## Questions?

Open a GitHub Discussion.
