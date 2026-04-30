# Encrypted Checkpoint Tags Design

## Context

`encrypttags` is an external hymx VM test project for encrypted tag end-to-end coverage. It already verifies that encrypted spawn and message tags reach the echo VM as decrypted params, while raw public messages keep encrypted tag values.

The remaining checkpoint gap is twofold:

- checkpoint files must store encrypted spawn tag metadata rather than plaintext secrets;
- after a node restart restores from checkpoint, the encrypted spawn tag must still decrypt into the VM env so later messages can use the restored `SpawnSecret`.

Hymx writes checkpoint files during node shutdown. `node.Close()` calls `runCheckpoint()`, which saves `ckp/ckp-<checkpoint-id>.json` in the node process working directory. That means full checkpoint validation is a live-node workflow: run e2e, stop the node to generate checkpoint, inspect the checkpoint file, restart the node, then verify restored behavior.

## Goals

- Verify checkpoint files contain encrypted spawn tag information for `Hymx-Encrypted-SpawnSecret`.
- Verify checkpoint files do not contain `spawn-secret-e2e` or `message-secret-e2e` plaintext.
- Verify checkpoint files do not store a plaintext `SpawnSecret` tag/value pair.
- Verify a restarted node can restore the process and still expose decrypted `SpawnSecret` to the echo VM.
- Keep ordinary `go test ./...` independent from Redis, ports, and a running node.

## Non-Goals

- Do not make checkpoint generation automatic outside hymx node shutdown semantics.
- Do not build a full process manager that starts and stops the node in tests.
- Do not change hymx production code for this task.
- Do not commit static plaintext-secret checkpoint fixture files. Tests may construct synthetic checkpoint data in memory or temporary directories.

## Approach

Use two layers:

1. **Offline checkpoint parsing helpers and unit tests** in `encrypttags`.
   These tests parse synthetic or temporary checkpoint files and validate the exact encrypted-tag and no-leak rules without requiring Redis or a live node.

2. **Manual live-node e2e commands** for the real checkpoint lifecycle.
   One command validates the latest generated checkpoint file for a spawned process. Another command verifies that, after node restart and recovery, the restored echo VM still sees the decrypted spawn secret.

This keeps regular tests stable while still giving a realistic workflow for checkpoint generation and restore.

## Checkpoint Content Validation

Add `examples/checkpoint.go` with helpers that can be unit-tested:

- `checkpointDir()` reads `ENCRYPTTAGS_CKP_DIR`, defaulting to `./ckp`.
- `loadCheckpointItem(path string)` reads a `ckp-*.json` file into `goarSchema.BundleItem`.
- `decodeCheckpointSnapshot(item goarSchema.BundleItem)` base64-decodes `item.Data` and unmarshals `vmmSchema.Snapshot`.
- `findCheckpointForProcess(dir, processID string)` scans checkpoint files and returns the checkpoint whose decoded snapshot has `snapshot.Env.Meta.Pid == processID`.
- `expectedCheckpointKeyType()` reads `ENCRYPTTAGS_KEY_TYPE`; when unset, it derives the key type from the local example SDK signer type.
- `verifyEncryptedCheckpoint(snapshot vmmSchema.Snapshot, rawSnapshotJSON []byte, processID, keyType string)` returns a structured result:
  - `Encrypted == true` when `snapshot.Env.Process.Tags` contains `Hymx-Encrypted-SpawnSecret`;
  - the encrypted tag value starts with `hymxenc:v1:<key-type>:`;
  - no `SpawnSecret` plaintext tag exists with value `spawn-secret-e2e`;
  - raw snapshot JSON does not contain `spawn-secret-e2e` or `message-secret-e2e`;
  - `snapshot.Env.Meta.Params` does not contain plaintext `SpawnSecret`.

Add command:

```bash
go run ./examples checkpoint <pid>
```

Behavior:

- reads from `ENCRYPTTAGS_CKP_DIR` or `./ckp`;
- determines expected key type from `ENCRYPTTAGS_KEY_TYPE` or local signer type;
- finds the checkpoint matching `<pid>`;
- validates encrypted checkpoint content;
- prints:

```text
CHECKPOINT encrypted=true plaintext_leaked=false
```

If no matching checkpoint exists, or encrypted spawn tag metadata is missing, the command exits non-zero with a clear error.

## Restore Usability Validation

Add command:

```bash
go run ./examples checkpoint-restore <pid>
```

This command assumes:

- the node was previously stopped after `go run ./examples encrypttags`, so checkpoint files were written;
- the node has been restarted and completed normal recovery;
- Redis still contains the checkpoint index for the process.

Behavior:

- sends a new message to `<pid>` with:
  - `Hymx-Encrypted-Secret = message-secret-e2e`;
  - `Plain = plain-e2e`;
- waits for result via existing SDK helpers;
- decodes echo output;
- verifies:
  - `SpawnSecret == spawn-secret-e2e`;
  - `Secret == message-secret-e2e`;
  - `Plain == plain-e2e`;
- prints:

```text
CHECKPOINT restore_decrypted=true
```

This proves the restored process env still has the decrypted spawn secret derived from the encrypted checkpoint process tags.

## Existing E2E Command Output

Update `go run ./examples encrypttags` output to include the process id:

```text
PROCESS pid=<spawnRes.Id>
```

This lets users copy the pid into:

```bash
go run ./examples checkpoint <pid>
go run ./examples checkpoint-restore <pid>
```

The existing success summary remains redacted:

```text
RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=plain-e2e
```

## Manual Workflow

1. Start Redis.
2. Start the node from the `encrypttags` repo root:

   ```bash
   go run ./cmd -c ./cmd/config.yaml
   ```

3. Initialize core token/registry if needed:

   ```bash
   go run ./examples init
   ```

4. Run encrypted tag e2e and record the printed pid:

   ```bash
   go run ./examples encrypttags
   ```

5. Stop the node with interrupt so it runs `Close()` and writes `./ckp/ckp-*.json`.
6. Validate checkpoint contents:

   ```bash
   go run ./examples checkpoint <pid>
   ```

7. Restart the node:

   ```bash
   go run ./cmd -c ./cmd/config.yaml
   ```

8. Validate restored encrypted spawn tag usability:

   ```bash
   go run ./examples checkpoint-restore <pid>
   ```

Expected combined evidence:

```text
CHECKPOINT encrypted=true plaintext_leaked=false
CHECKPOINT restore_decrypted=true
```

## Unit Test Coverage

Add `examples/checkpoint_test.go` covering:

- a valid checkpoint snapshot with `Hymx-Encrypted-SpawnSecret` passes validation;
- checkpoint JSON containing `spawn-secret-e2e` fails as plaintext leakage;
- checkpoint JSON containing `message-secret-e2e` fails as plaintext leakage;
- a plaintext `SpawnSecret` tag fails validation;
- missing `Hymx-Encrypted-SpawnSecret` fails validation;
- wrong cipher key type prefix fails validation;
- process id mismatch fails lookup/validation;
- malformed checkpoint file returns a clear error.

These tests should use temporary directories and synthetic checkpoint items. They must not require Redis or a running node.

## Validation

Run ordinary repository checks:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./...
go build -o ./build/hymx-node ./cmd
```

Manual live workflow requires Redis and a running node:

```bash
go run ./cmd -c ./cmd/config.yaml
go run ./examples init
go run ./examples encrypttags
# stop node to write checkpoint
go run ./examples checkpoint <pid>
# restart node
go run ./examples checkpoint-restore <pid>
```

## Acceptance Criteria

- `checkpoint <pid>` proves the checkpoint file contains encrypted `Hymx-Encrypted-SpawnSecret` metadata.
- `checkpoint <pid>` proves checkpoint data does not contain plaintext spawn/message secrets.
- `checkpoint-restore <pid>` proves a restarted node restores the encrypted spawn tag into usable decrypted VM state.
- `go test ./...` passes without Redis or a running node.
- `go build -o ./build/hymx-node ./cmd` passes.
