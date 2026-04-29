# Encrypttags Test Coverage Design

## Context

`encrypttags` is an external hymx VM test project for encrypted tag end-to-end coverage. It mounts a small echo VM into a hymx node and verifies that encrypted spawn and message tags are visible to hymx/VMM as decrypted params while raw public storage keeps ciphertext.

The current hymx implementation already has unit tests for basic tagcrypto round trips, SDK redirect re-encryption, node internal decryption, checkpoint sanitization, and VMM suppression of encrypted `X-` params. The current encrypttags project has a single local e2e flow and a small echo VM that confirms one spawn secret, one message secret, one plain tag, raw item ciphertext, and reserved tag rejection.

## Goals

- Strengthen coverage for encrypted tag safety boundaries.
- Add negative input coverage for malformed encrypted tag names, ciphertext values, metadata, and signer/key mismatches.
- Keep hymx production code unchanged for this task.
- Allow broader changes inside `encrypttags`, including test helpers and richer echo VM behavior.

## Non-Goals

- Do not redesign encrypted tag semantics.
- Do not move code between repositories.
- Do not introduce a mock hymx implementation inside `encrypttags`.
- Do not require Docker, Redis, or a running node for ordinary unit tests.

## Repository Boundaries

Primary changes belong in:

- `/Users/webbergao/work/src/HymxWorkspace/encrypttags`

Allowed hymx changes are limited to test files:

- `/Users/webbergao/work/src/HymxWorkspace/hymx/utils/tagcrypto/*_test.go`
- `/Users/webbergao/work/src/HymxWorkspace/hymx/sdk/*_test.go`
- `/Users/webbergao/work/src/HymxWorkspace/hymx/node/*_test.go`
- `/Users/webbergao/work/src/HymxWorkspace/hymx/vmm/*_test.go` if needed

Hymx production files should not be edited for this task.

## Approach

Use two layers of coverage.

The `encrypttags` repository remains the acceptance layer. It should verify realistic external VM behavior with hymx as a dependency through the existing `replace github.com/hymatrix/hymx => ../hymx` rule. Its tests should exercise the echo VM and helper functions without requiring a live node, while the manual `go run ./examples encrypttags` flow remains the local live-node e2e validation.

The `hymx` repository supplies narrow unit tests for edge cases that are hard to drive through a live external VM. These tests should focus on deterministic failures and security boundaries in `tagcrypto`, SDK encryption metadata handling, and node decryption/checkpoint behavior.

## Encrypttags Coverage

### Echo VM Unit Tests

Extend the echo VM tests to cover:

- spawn-time decrypted encrypted params are readable by the VM;
- message-time decrypted encrypted params are returned in output;
- plain params continue to pass through unchanged;
- missing encrypted params produce empty output fields rather than panics;
- checkpoint payloads do not contain spawn or message secrets;
- restore keeps spawn secrets supplied by recovered hymx env rather than loading secrets from checkpoint data.

If needed, extend the echo VM output map with stable fields that make these checks direct. Keep output names explicit and deterministic.

### Example Helper Tests

Extend `examples/encrypttags_test.go` to cover:

- `encryptedTagStored` detects ciphertext with the expected `hymxenc:v1:<key-type>:` prefix;
- plaintext leakage is detected when the encrypted tag value equals the secret;
- plaintext leakage is detected when any unrelated raw tag value equals the secret;
- wrong key type prefix does not count as encrypted;
- missing encrypted tag does not count as encrypted;
- `outputMap` accepts a normal `map[string]interface{}` of strings;
- `outputMap` rejects non-map output and non-string values.

### Manual E2E Flow

Keep `go run ./examples encrypttags` as the live-node acceptance command. Expand its assertions to check:

- raw spawn item stores `Hymx-Encrypted-SpawnSecret` as ciphertext and not plaintext;
- raw message item stores `Hymx-Encrypted-Secret` as ciphertext and not plaintext;
- plain message tag remains visible as plain output;
- decrypted spawn and message secrets reach the echo VM output;
- encrypted reserved tag submission returns a client/server error;
- success output redacts secret values and prints only non-sensitive status.

The command may print key type and public key because those are advertised node metadata, but it must not print plaintext secret values except the existing non-sensitive plain sentinel.

## Hymx Unit Coverage

### tagcrypto

Add table-driven tests for:

- empty encrypted tag name: `Hymx-Encrypted-`;
- non-encrypted tags return `changed=false` and preserve order/value;
- mixed plain and encrypted tags preserve plain tags and decrypt encrypted tags to plain names;
- `EncryptedPlainTagNames` returns only decrypted names for encrypted tags;
- malformed cipher values fail decrypt;
- unsupported key type fails encrypt;
- unsupported cipher key type fails decrypt;
- wrong signer type fails decrypt;
- encrypted reserved names are rejected consistently by validation, encryption, and encrypted-name extraction.

### SDK

Add tests for:

- encrypted send fails when `/info` omits `Encryption-Public-Key`;
- encrypted send fails when `/info` omits `Encryption-Key-Type`;
- plain send does not fetch `/info`;
- encrypted redirect skips unusable redirected nodes and returns an error when no usable redirected node succeeds;
- rejected encrypted protocol tags fail before network access.

These tests should use `httptest` and decode submitted bundle items only where required to assert raw tag shape.

### Node

Add tests for:

- decrypting an encrypted message with malformed ciphertext fails before producing an internal instance;
- decrypting with a signer that does not match the ciphertext key type fails;
- `sanitizeCheckpointSnapshot` fails when the raw spawn item type is not `Process`;
- `decryptSnapshotEnv` fails on malformed encrypted process tags and leaves no successful decrypted state.

Use existing node test doubles where possible and add small `_test.go` helpers only when they reduce repeated signer/item setup.

## Validation

Run cheap package checks first:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./...
go build -o ./build/hymx-node ./cmd
```

Then run the focused hymx tests:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
go test ./utils/tagcrypto ./sdk ./node ./vmm/...
```

If focused checks pass and time allows, run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
go test ./...
```

The live local acceptance flow remains optional because it requires Redis and a running hymx node:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go run ./cmd -c ./cmd/config.yaml
go run ./examples init
go run ./examples encrypttags
```

## Acceptance Criteria

- `encrypttags` has broader unit coverage for echo VM behavior and helper leakage detection.
- `hymx` has deterministic unit tests for encrypted tag negative inputs and safety boundaries.
- No hymx production files are changed.
- Test names clearly describe the encrypted-tag safety property being checked.
- Verification commands and any blocked checks are reported before implementation is considered complete.
