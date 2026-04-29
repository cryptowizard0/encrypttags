# encrypttags

`encrypttags` is a standalone hymx test VM project for encrypted tag e2e coverage. It mounts a small external echo VM into a hymx node and runs a local flow that verifies encrypted spawn/message tags are only visible to hymx and the VMM as decrypted params.

## Requirements

- Go 1.24.x
- Redis on `localhost:6379`
- A local checkout of `hymx` at `../hymx`

The module keeps:

```go
replace github.com/hymatrix/hymx => ../hymx
```

so it can test the unmerged `feature/encrypt_tags` branch.

## Run

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
RAW spawn encrypted=true plaintext_leaked=false
RAW message encrypted=true plaintext_leaked=false
reserved encrypted tag rejected=true
```

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
