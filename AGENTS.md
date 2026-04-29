# AGENTS.md

This file defines repo-local agent guidance for `encrypttags`.

## Base Rules

Apply the shared workspace rules from:

- `/Users/webbergao/work/src/HymxWorkspace/AGENTS.md`

Also follow the shared Go coding standard from:

- `/Users/webbergao/work/src/HymxWorkspace/docs/golang-coding-standards.md`

## Repository Purpose

`encrypttags` is an external hymx VM test project for encrypted tag e2e coverage. It should stay independent from `hymx` core packages, like `vmdocker`.

## Validation

For code changes in this repo, prefer:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./...
go build -o ./build/hymx-node ./cmd
```
