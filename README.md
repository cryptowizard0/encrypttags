# encrypttags

`encrypttags` is a standalone hymx test VM project for encrypted tag e2e coverage. It mounts a small external echo VM into a hymx node and runs a local flow that verifies encrypted message tags are stored as ciphertext and reach the VM as decrypted params.

## Requirements

- Go 1.24.x
- Redis on `localhost:6379`
- A local checkout of `hymx` at `../hymx`

The module keeps:

```go
replace github.com/hymatrix/hymx => ../hymx
```

so it can test the local `feature/cryptor` branch.

## Run

Run the full local Redis/node/checkpoint flow automatically:

```bash
./scripts/encrypttags-e2e.sh
```

The script recreates the `hype-vmdocker-redis` container, starts the node in the background, initializes token/registry, runs `encrypttags`, stops the node to write checkpoints, validates checkpoint content, restarts the node, and runs `checkpoint-restore`.

Logs are saved under `.tmp/encrypttags-e2e/<timestamp>/`. To preview the commands without changing Redis or starting the node:

```bash
./scripts/encrypttags-e2e.sh --dry-run
```

Useful overrides:

```bash
REDIS_CONTAINER=hype-vmdocker-redis \
NODE_READY_TIMEOUT=90 \
LOG_DIR=/tmp/encrypttags-e2e \
./scripts/encrypttags-e2e.sh
```

Manual flow:

Start the node:

```bash
go run ./cmd -c ./cmd/config.yaml
```

In another terminal, run the e2e flow:

```bash
go run ./examples init
go run ./examples encrypttags
```

The `init` command manually initializes the local core token/registry VMs, matching `hymx/examples/init.go`. The `encrypttags` command expects token/registry to already exist, then generates `mod/mod-<id>.json` automatically when `ENCRYPTTAGS_MODULE_ID` is not set. To reuse an echo module id:

```bash
go run ./examples module
ENCRYPTTAGS_MODULE_ID=<module-id> go run ./examples encrypttags
```

Expected success output includes:

```text
E2E encrypted tags passed
RAW message encrypted=true plaintext_leaked=false
PROCESS pid=<process-id>
RESULT decrypted=true Secret=<redacted> Plain=plain-e2e
```

## Checkpoint Validation

Checkpoint files are written when the node shuts down. After `go run ./examples encrypttags` prints `PROCESS pid=<process-id>`, stop the running node with interrupt so it writes `ckp/ckp-*.json`, then validate that the generated checkpoint file does not contain plaintext encrypted-message sentinels:

```bash
go run ./examples checkpoint <process-id>
```

Expected output:

```text
CHECKPOINT plaintext_leaked=false
```

This checks the latest matching `ckp/ckp-*.json` bundle item, decodes the checkpoint snapshot, and fails if the encrypted message sentinel appears in checkpoint JSON as plaintext.

After restarting the node with the checkpoint present, verify the restored process can still decrypt a new encrypted message tag:

```bash
go run ./examples checkpoint-restore <process-id>
```

Expected output:

```text
CHECKPOINT restore_decrypted=true
```

By default, the checkpoint command searches the common local run directories, including `./ckp`, `../ckp`, `cmd/ckp`, and `../cmd/ckp`, so it works whether you run examples from the repo root or from `examples/` after starting the node from `cmd/`. Use `ENCRYPTTAGS_CKP_DIR` to inspect a specific checkpoint directory.

## Configuration

Defaults are local-test oriented:

- Node URL: `http://127.0.0.1:8080`
- Redis DB: `15`
- Example private key: local test key only

Override example client settings with:

```bash
ENCRYPTTAGS_URL=http://127.0.0.1:8080
ENCRYPTTAGS_PRIVATE_KEY=0x...
ENCRYPTTAGS_MODULE_ID=<module-id>
```

## Validation

```bash
go test ./...
go build -o ./build/hymx-node ./cmd
```
