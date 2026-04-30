# Encrypted Checkpoint Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `encrypttags` checkpoint validation commands and unit tests that prove checkpoint files retain encrypted spawn tag metadata, do not leak plaintext secrets, and restored processes can still use decrypted encrypted tags.

**Architecture:** Keep ordinary tests offline by adding checkpoint parsing and validation helpers under `examples/`. Add live-node CLI commands for the real checkpoint lifecycle: one command checks generated checkpoint files after node shutdown, and one command verifies restored VM behavior after node restart. Do not modify `hymx` production code.

**Tech Stack:** Go 1.24, `testing`, `stretchr/testify/require`, `github.com/hymatrix/hymx/vmm/schema`, `github.com/hymatrix/hymx/utils/tagcrypto`, `github.com/permadao/goar/schema`, `github.com/permadao/goar/utils`.

---

## File Structure

Create:

- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go`
  - Owns checkpoint file discovery, checkpoint item parsing, snapshot decoding, encrypted checkpoint validation, and the `checkpoint` / `checkpoint-restore` command functions.
- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go`
  - Offline tests for checkpoint validation helpers using temporary checkpoint files.

Modify:

- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/main.go`
  - Adds `checkpoint` and `checkpoint-restore` subcommands.
- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`
  - Prints `PROCESS pid=<pid>` and reuses a small echo-output verification helper.
- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/README.md`
  - Documents the manual checkpoint content and restore validation workflow.

No `hymx` files should be edited.

---

### Task 1: Add Offline Checkpoint Validation Helpers

**Files:**
- Create: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go`
- Create: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go`

- [ ] **Step 1: Write failing checkpoint helper tests**

Create `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go` with:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/require"
)

const checkpointTestPid = "process-id"

func TestVerifyEncryptedCheckpointAcceptsEncryptedSpawnTag(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, map[string]string{"Plain": "plain-e2e"})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.NoError(t, err)
	require.True(t, result.Encrypted)
	require.False(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsSpawnSecretLeak(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, map[string]string{"Debug": e2eSpawnSecret})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsMessageSecretLeak(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, map[string]string{"Debug": e2eMessageSecret})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsPlainSpawnSecretTag(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: "SpawnSecret", Value: e2eSpawnSecret},
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsMissingEncryptedSpawnTag(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: "Plain", Value: "plain-e2e"},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.Error(t, err)
	require.False(t, result.Encrypted)
}

func TestVerifyEncryptedCheckpointRejectsWrongCipherKeyType(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeArweaveRSAOAEP)},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid, tagcrypto.KeyTypeEthereumECIES)

	require.Error(t, err)
	require.False(t, result.Encrypted)
}

func TestFindCheckpointForProcessSelectsMatchingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-other.json", checkpointItemForTest(t, checkpointSnapshotForTest("other-process", []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, nil)))
	expectedPath := writeCheckpointFileForTest(t, dir, "ckp-process.json", checkpointItemForTest(t, checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, nil)))

	found, err := findCheckpointForProcess(dir, checkpointTestPid)

	require.NoError(t, err)
	require.Equal(t, expectedPath, found.Path)
	require.Equal(t, checkpointTestPid, found.Snapshot.Env.Meta.Pid)
}

func TestFindCheckpointForProcessRejectsMismatchedProcess(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-other.json", checkpointItemForTest(t, checkpointSnapshotForTest("other-process", []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, nil)))

	_, err := findCheckpointForProcess(dir, checkpointTestPid)

	require.Error(t, err)
	require.Contains(t, err.Error(), "checkpoint not found")
}

func TestLoadCheckpointItemRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ckp-bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0644))

	_, err := loadCheckpointItem(path)

	require.Error(t, err)
}

func TestExpectedCheckpointKeyTypeUsesEnvOverride(t *testing.T) {
	t.Setenv("ENCRYPTTAGS_KEY_TYPE", tagcrypto.KeyTypeArweaveRSAOAEP)

	keyType, err := expectedCheckpointKeyType()

	require.NoError(t, err)
	require.Equal(t, tagcrypto.KeyTypeArweaveRSAOAEP, keyType)
}

func checkpointSnapshotForTest(pid string, tags []goarSchema.Tag, params map[string]string) vmmSchema.Snapshot {
	if params == nil {
		params = map[string]string{}
	}
	return vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid:    pid,
				Params: params,
			},
			Process: hymxSchema.Process{Tags: tags},
		},
	}
}

func checkpointSnapshotJSON(t *testing.T, snap vmmSchema.Snapshot) []byte {
	t.Helper()
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	return raw
}

func checkpointItemForTest(t *testing.T, snap vmmSchema.Snapshot) goarSchema.BundleItem {
	t.Helper()
	return goarSchema.BundleItem{
		Id:   "checkpoint-id",
		Data: goarUtils.Base64Encode(checkpointSnapshotJSON(t, snap)),
	}
}

func writeCheckpointFileForTest(t *testing.T, dir, name string, item goarSchema.BundleItem) string {
	t.Helper()
	path := filepath.Join(dir, name)
	by, err := json.Marshal(item)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, by, 0644))
	require.NoError(t, os.Chtimes(path, time.Now(), time.Now()))
	return path
}

func checkpointCipherValue(keyType string) string {
	return tagcrypto.CipherValuePrefix + ":" + keyType + ":ciphertext"
}
```

- [ ] **Step 2: Run tests to verify they fail before implementation**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run 'TestVerifyEncryptedCheckpoint|TestFindCheckpointForProcess|TestLoadCheckpointItem|TestExpectedCheckpointKeyType' -count=1
```

Expected: FAIL with undefined helper names such as `verifyEncryptedCheckpoint`, `findCheckpointForProcess`, `loadCheckpointItem`, and `expectedCheckpointKeyType`.

- [ ] **Step 3: Implement checkpoint helpers**

Create `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go` with:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

type checkpointMatch struct {
	Path            string
	Item            goarSchema.BundleItem
	Snapshot        vmmSchema.Snapshot
	RawSnapshotJSON []byte
	modTime         time.Time
}

type checkpointVerification struct {
	Encrypted       bool
	PlaintextLeaked bool
}

func checkpointDir() string {
	if dir := os.Getenv("ENCRYPTTAGS_CKP_DIR"); dir != "" {
		return dir
	}
	return "./ckp"
}

func expectedCheckpointKeyType() (string, error) {
	if keyType := os.Getenv("ENCRYPTTAGS_KEY_TYPE"); keyType != "" {
		return keyType, nil
	}
	initSDK()
	return tagcrypto.KeyTypeFromSignatureType(s.Bundler.SignType)
}

func loadCheckpointItem(path string) (goarSchema.BundleItem, error) {
	by, err := os.ReadFile(path)
	if err != nil {
		return goarSchema.BundleItem{}, fmt.Errorf("read checkpoint file %s: %w", path, err)
	}
	var item goarSchema.BundleItem
	if err := json.Unmarshal(by, &item); err != nil {
		return goarSchema.BundleItem{}, fmt.Errorf("decode checkpoint file %s: %w", path, err)
	}
	return item, nil
}

func decodeCheckpointSnapshot(item goarSchema.BundleItem) (vmmSchema.Snapshot, []byte, error) {
	raw, err := goarUtils.Base64Decode(item.Data)
	if err != nil {
		return vmmSchema.Snapshot{}, nil, fmt.Errorf("decode checkpoint snapshot data: %w", err)
	}
	var snap vmmSchema.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return vmmSchema.Snapshot{}, nil, fmt.Errorf("decode checkpoint snapshot json: %w", err)
	}
	return snap, raw, nil
}

func findCheckpointForProcess(dir, processID string) (checkpointMatch, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "ckp-*.json"))
	if err != nil {
		return checkpointMatch{}, fmt.Errorf("list checkpoint files: %w", err)
	}
	sort.Strings(paths)

	var best checkpointMatch
	for _, path := range paths {
		item, err := loadCheckpointItem(path)
		if err != nil {
			return checkpointMatch{}, err
		}
		snap, raw, err := decodeCheckpointSnapshot(item)
		if err != nil {
			return checkpointMatch{}, fmt.Errorf("decode checkpoint %s: %w", path, err)
		}
		if snap.Env.Meta.Pid != processID {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return checkpointMatch{}, fmt.Errorf("stat checkpoint file %s: %w", path, err)
		}
		candidate := checkpointMatch{
			Path:            path,
			Item:            item,
			Snapshot:        snap,
			RawSnapshotJSON: raw,
			modTime:         info.ModTime(),
		}
		if best.Path == "" || candidate.modTime.After(best.modTime) {
			best = candidate
		}
	}
	if best.Path == "" {
		return checkpointMatch{}, fmt.Errorf("checkpoint not found for process %s in %s", processID, dir)
	}
	return best, nil
}

func verifyEncryptedCheckpoint(snapshot vmmSchema.Snapshot, rawSnapshotJSON []byte, processID, keyType string) (checkpointVerification, error) {
	result := checkpointVerification{}
	if snapshot.Env.Meta.Pid != processID {
		return result, fmt.Errorf("checkpoint process mismatch: got %s want %s", snapshot.Env.Meta.Pid, processID)
	}
	rawSnapshot := string(rawSnapshotJSON)
	if strings.Contains(rawSnapshot, e2eSpawnSecret) || strings.Contains(rawSnapshot, e2eMessageSecret) {
		result.PlaintextLeaked = true
		return result, fmt.Errorf("checkpoint plaintext secret leaked")
	}
	if snapshot.Env.Meta.Params["SpawnSecret"] == e2eSpawnSecret {
		result.PlaintextLeaked = true
		return result, fmt.Errorf("checkpoint meta params contain plaintext SpawnSecret")
	}

	expectedPrefix := tagcrypto.CipherValuePrefix + ":" + keyType + ":"
	for _, tag := range snapshot.Env.Process.Tags {
		if tag.Name == "SpawnSecret" && tag.Value == e2eSpawnSecret {
			result.PlaintextLeaked = true
			return result, fmt.Errorf("checkpoint process tags contain plaintext SpawnSecret")
		}
		if tag.Name != tagcrypto.EncryptedTagPrefix+"SpawnSecret" {
			continue
		}
		if !strings.HasPrefix(tag.Value, expectedPrefix) {
			return result, fmt.Errorf("encrypted SpawnSecret has unexpected key type")
		}
		result.Encrypted = true
	}
	if !result.Encrypted {
		return result, fmt.Errorf("checkpoint missing encrypted SpawnSecret tag")
	}
	return result, nil
}
```

- [ ] **Step 4: Run helper tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
gofmt -w examples/checkpoint.go examples/checkpoint_test.go
go test ./examples -run 'TestVerifyEncryptedCheckpoint|TestFindCheckpointForProcess|TestLoadCheckpointItem|TestExpectedCheckpointKeyType' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add examples/checkpoint.go examples/checkpoint_test.go
git commit -m "test: add encrypted checkpoint validation helpers"
```

---

### Task 2: Wire Checkpoint CLI Commands

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/main.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go`

- [ ] **Step 1: Add CLI command tests**

Append these tests to `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go`:

```go
func TestCheckpointCmdPrintsEncryptedStatus(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-process.json", checkpointItemForTest(t, checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: checkpointCipherValue(tagcrypto.KeyTypeEthereumECIES)},
	}, nil)))
	t.Setenv("ENCRYPTTAGS_CKP_DIR", dir)
	t.Setenv("ENCRYPTTAGS_KEY_TYPE", tagcrypto.KeyTypeEthereumECIES)
	var buf bytes.Buffer

	err := checkpointCmd(&buf, []string{checkpointTestPid})

	require.NoError(t, err)
	require.Contains(t, buf.String(), "CHECKPOINT encrypted=true plaintext_leaked=false")
}

func TestCheckpointCmdRequiresPid(t *testing.T) {
	var buf bytes.Buffer

	err := checkpointCmd(&buf, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}
```

Also add `bytes` to the existing import block in `checkpoint_test.go`.

- [ ] **Step 2: Run CLI tests to verify they fail before command implementation**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run 'TestCheckpointCmd' -count=1
```

Expected: FAIL with `undefined: checkpointCmd`.

- [ ] **Step 3: Implement `checkpointCmd`**

Append this function to `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go`:

```go
func checkpointCmd(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: checkpoint <pid>")
	}
	keyType, err := expectedCheckpointKeyType()
	if err != nil {
		return fmt.Errorf("determine checkpoint key type: %w", err)
	}
	match, err := findCheckpointForProcess(checkpointDir(), args[0])
	if err != nil {
		return err
	}
	result, err := verifyEncryptedCheckpoint(match.Snapshot, match.RawSnapshotJSON, args[0], keyType)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "CHECKPOINT encrypted=%v plaintext_leaked=%v\n", result.Encrypted, result.PlaintextLeaked)
	return nil
}
```

Add `io` to the import block in `checkpoint.go`.

- [ ] **Step 4: Wire command in `examples/main.go`**

Change the usage text:

```go
fmt.Println("please input cmd, ex: init, module, encrypttags, checkpoint, checkpoint-restore")
```

Add this switch case after `encrypttags`:

```go
case "checkpoint":
	if err := checkpointCmd(os.Stdout, os.Args[2:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
```

- [ ] **Step 5: Run command tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
gofmt -w examples/main.go examples/checkpoint.go examples/checkpoint_test.go
go test ./examples -run 'TestCheckpointCmd' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add examples/main.go examples/checkpoint.go examples/checkpoint_test.go
git commit -m "feat: add encrypted checkpoint inspection command"
```

---

### Task 3: Add Restore Validation Command

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/main.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go`

- [ ] **Step 1: Add echo result verification helper tests**

Append these tests to `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/checkpoint_test.go`:

```go
func TestVerifyEchoMessageAcceptsExpectedOutput(t *testing.T) {
	message := echoResultMessageForTest(t, map[string]string{
		"SpawnSecret": e2eSpawnSecret,
		"Secret":      e2eMessageSecret,
		"Plain":       e2ePlain,
	})

	output, err := verifyEchoMessage(message)

	require.NoError(t, err)
	require.Equal(t, e2eSpawnSecret, output["SpawnSecret"])
	require.Equal(t, e2eMessageSecret, output["Secret"])
	require.Equal(t, e2ePlain, output["Plain"])
}

func TestVerifyEchoMessageRejectsUnexpectedOutput(t *testing.T) {
	message := echoResultMessageForTest(t, map[string]string{
		"SpawnSecret": "missing-after-restore",
		"Secret":      e2eMessageSecret,
		"Plain":       e2ePlain,
	})

	_, err := verifyEchoMessage(message)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected echo output")
}

func echoResultMessageForTest(t *testing.T, output map[string]string) string {
	t.Helper()
	by, err := json.Marshal(vmmSchema.VmmResult{Output: output})
	require.NoError(t, err)
	return string(by)
}
```

- [ ] **Step 2: Run tests to verify helper is missing**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run 'TestVerifyEchoMessage' -count=1
```

Expected: FAIL with `undefined: verifyEchoMessage`.

- [ ] **Step 3: Implement `verifyEchoMessage` and reuse it**

In `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`, add:

```go
func verifyEchoMessage(message string) (map[string]string, error) {
	var result vmmSchema.VmmResult
	if err := json.Unmarshal([]byte(message), &result); err != nil {
		return nil, fmt.Errorf("decode message result: %w", err)
	}
	output, err := outputMap(result.Output)
	if err != nil {
		return nil, err
	}
	if output["SpawnSecret"] != e2eSpawnSecret || output["Secret"] != e2eMessageSecret || output["Plain"] != e2ePlain {
		return nil, fmt.Errorf("unexpected echo output: decrypted values did not match expected sentinels")
	}
	return output, nil
}
```

Replace the existing result decode block in `encryptTagsCmd`:

```go
var result vmmSchema.VmmResult
if err := json.Unmarshal([]byte(msgRes.Message), &result); err != nil {
	return fmt.Errorf("decode message result: %w", err)
}
output, err := outputMap(result.Output)
if err != nil {
	return err
}
if output["SpawnSecret"] != e2eSpawnSecret || output["Secret"] != e2eMessageSecret || output["Plain"] != e2ePlain {
	return fmt.Errorf("unexpected echo output: decrypted values did not match expected sentinels")
}
```

with:

```go
output, err := verifyEchoMessage(msgRes.Message)
if err != nil {
	return err
}
```

- [ ] **Step 4: Add restore command tests for argument validation**

Append this test to `checkpoint_test.go`:

```go
func TestCheckpointRestoreCmdRequiresPid(t *testing.T) {
	var buf bytes.Buffer

	err := checkpointRestoreCmd(&buf, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}
```

- [ ] **Step 5: Run restore command test to verify command is missing**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run 'TestCheckpointRestoreCmdRequiresPid' -count=1
```

Expected: FAIL with `undefined: checkpointRestoreCmd`.

- [ ] **Step 6: Implement `checkpointRestoreCmd`**

Append this function to `examples/checkpoint.go`:

```go
func checkpointRestoreCmd(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: checkpoint-restore <pid>")
	}
	msgRes, err := s.SendMessageAndWait(args[0], "", []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: e2eMessageSecret},
		{Name: "Plain", Value: e2ePlain},
	})
	if err != nil {
		return fmt.Errorf("send restore check message: %w", err)
	}
	if _, err := verifyEchoMessage(msgRes.Message); err != nil {
		return err
	}
	fmt.Fprintln(w, "CHECKPOINT restore_decrypted=true")
	return nil
}
```

Add `goarSchema "github.com/permadao/goar/schema"` to `checkpoint.go` imports if it is not already present.

- [ ] **Step 7: Wire `checkpoint-restore` in `examples/main.go`**

Add this switch case after `checkpoint`:

```go
case "checkpoint-restore":
	if err := checkpointRestoreCmd(os.Stdout, os.Args[2:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
```

- [ ] **Step 8: Run examples tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
gofmt -w examples/main.go examples/checkpoint.go examples/checkpoint_test.go examples/encrypttags.go
go test ./examples -run 'TestVerifyEchoMessage|TestCheckpointRestoreCmdRequiresPid' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 3**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add examples/main.go examples/checkpoint.go examples/checkpoint_test.go examples/encrypttags.go
git commit -m "feat: add encrypted checkpoint restore check"
```

---

### Task 4: Print Process ID From Existing E2E Flow

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags_test.go`

- [ ] **Step 1: Extend success summary test to include process id**

Update `TestPrintEncryptedTagsSuccessRedactsSecrets` in `examples/encrypttags_test.go` so the summary includes `ProcessID`:

```go
printEncryptedTagsSuccess(&buf, encryptedTagsSummary{
	ProcessID:        "process-id",
	SpawnEncrypted:   true,
	SpawnLeaked:      false,
	MessageEncrypted: true,
	MessageLeaked:    false,
	ReservedRejected: true,
	Plain:            "plain-e2e",
})
```

Add this assertion after `output := buf.String()`:

```go
require.Contains(t, output, "PROCESS pid=process-id")
```

- [ ] **Step 2: Run test to verify it fails before implementation**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run TestPrintEncryptedTagsSuccessRedactsSecrets -count=1
```

Expected: FAIL with `unknown field ProcessID` or missing `PROCESS pid=process-id`.

- [ ] **Step 3: Add process id to summary output**

In `examples/encrypttags.go`, update `encryptedTagsSummary`:

```go
type encryptedTagsSummary struct {
	ProcessID        string
	SpawnEncrypted   bool
	SpawnLeaked      bool
	MessageEncrypted bool
	MessageLeaked    bool
	ReservedRejected bool
	Plain            string
}
```

Update `printEncryptedTagsSuccess` to print the process id after the first line:

```go
func printEncryptedTagsSuccess(w io.Writer, summary encryptedTagsSummary) {
	fmt.Fprintln(w, "E2E encrypted tags passed")
	if summary.ProcessID != "" {
		fmt.Fprintf(w, "PROCESS pid=%s\n", summary.ProcessID)
	}
	fmt.Fprintf(w, "RAW spawn encrypted=%v plaintext_leaked=%v\n", summary.SpawnEncrypted, summary.SpawnLeaked)
	fmt.Fprintf(w, "RAW message encrypted=%v plaintext_leaked=%v\n", summary.MessageEncrypted, summary.MessageLeaked)
	fmt.Fprintf(w, "RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=%s\n", summary.Plain)
	fmt.Fprintf(w, "reserved encrypted tag rejected=%v\n", summary.ReservedRejected)
}
```

In `encryptTagsCmd`, set `ProcessID`:

```go
printEncryptedTagsSuccess(os.Stdout, encryptedTagsSummary{
	ProcessID:        spawnRes.Id,
	SpawnEncrypted:   spawnEncrypted,
	SpawnLeaked:      spawnLeaked,
	MessageEncrypted: messageEncrypted,
	MessageLeaked:    messageLeaked,
	ReservedRejected: reservedRejected,
	Plain:            output["Plain"],
})
```

- [ ] **Step 4: Run examples tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
gofmt -w examples/encrypttags.go examples/encrypttags_test.go
go test ./examples -run TestPrintEncryptedTagsSuccessRedactsSecrets -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 4**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add examples/encrypttags.go examples/encrypttags_test.go
git commit -m "feat: print encrypted tag process id"
```

---

### Task 5: Document Manual Checkpoint Workflow

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/README.md`

- [ ] **Step 1: Add README checkpoint workflow**

In `README.md`, after the existing `Run` section, add:

```markdown
## Checkpoint encrypted tags

Checkpoint validation is a live-node workflow because hymx writes checkpoint files when the node shuts down.

1. Start Redis.
2. Start the node from the repo root:

   ```bash
   go run ./cmd -c ./cmd/config.yaml
   ```

3. Initialize token/registry if needed:

   ```bash
   go run ./examples init
   ```

4. Run encrypted tag e2e and copy the printed process id:

   ```bash
   go run ./examples encrypttags
   ```

5. Stop the node with interrupt so it writes `./ckp/ckp-*.json`.
6. Verify the checkpoint contains encrypted spawn tag metadata and no plaintext secrets:

   ```bash
   go run ./examples checkpoint <pid>
   ```

7. Restart the node:

   ```bash
   go run ./cmd -c ./cmd/config.yaml
   ```

8. Verify the restored process can still read the decrypted encrypted spawn tag:

   ```bash
   go run ./examples checkpoint-restore <pid>
   ```

Expected checkpoint evidence:

```text
CHECKPOINT encrypted=true plaintext_leaked=false
CHECKPOINT restore_decrypted=true
```

Use `ENCRYPTTAGS_CKP_DIR=/path/to/ckp` when checkpoint files are not under `./ckp`.
Use `ENCRYPTTAGS_KEY_TYPE=<key-type>` to override the expected checkpoint cipher key type.
```

- [ ] **Step 2: Run README-adjacent checks**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit Task 5**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add README.md
git commit -m "docs: add encrypted checkpoint workflow"
```

---

### Task 6: Final Verification

**Files:**
- Read: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/docs/superpowers/specs/2026-04-30-encrypted-checkpoint-tags-design.md`
- Read: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/docs/superpowers/plans/2026-04-30-encrypted-checkpoint-tags.md`

- [ ] **Step 1: Run ordinary repository checks**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./...
go build -o ./build/hymx-node ./cmd
```

Expected: both commands exit 0.

- [ ] **Step 2: Verify no generated checkpoint files are present**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git status --short
```

Expected: no untracked `ckp/` or `cmd/ckp/` checkpoint files. Remove generated checkpoint files if tests or manual checks created them.

- [ ] **Step 3: Record optional live workflow status**

Do not run this unless Redis is available and the user wants a live check:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go run ./cmd -c ./cmd/config.yaml
go run ./examples init
go run ./examples encrypttags
# stop node to write checkpoint
go run ./examples checkpoint <pid>
# restart node
go run ./examples checkpoint-restore <pid>
```

Expected manual evidence:

```text
CHECKPOINT encrypted=true plaintext_leaked=false
CHECKPOINT restore_decrypted=true
```

- [ ] **Step 4: Report final changed files**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git diff --name-only origin/main..HEAD
```

Expected changed files include:

```text
README.md
docs/superpowers/specs/2026-04-30-encrypted-checkpoint-tags-design.md
docs/superpowers/plans/2026-04-30-encrypted-checkpoint-tags.md
examples/checkpoint.go
examples/checkpoint_test.go
examples/encrypttags.go
examples/encrypttags_test.go
examples/main.go
```

---

## Self-Review Notes

- Spec coverage: Task 1 covers offline parsing and encrypted/no-leak validation. Task 2 covers `checkpoint <pid>`. Task 3 covers `checkpoint-restore <pid>`. Task 4 prints `PROCESS pid=<pid>`. Task 5 documents the live workflow. Task 6 covers ordinary and optional live validation.
- Placeholder scan: No `TBD`, `TODO`, or unspecified implementation steps remain in this plan.
- Type consistency: Function names are consistent across tasks: `checkpointCmd`, `checkpointRestoreCmd`, `verifyEncryptedCheckpoint`, `findCheckpointForProcess`, and `verifyEchoMessage`.
- Scope check: The implementation is limited to `encrypttags`. No `hymx` files are planned for modification.
