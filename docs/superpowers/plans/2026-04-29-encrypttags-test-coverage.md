# Encrypttags Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add stronger safety-boundary and malformed-input test coverage for hymx encrypted tags, with `encrypttags` as the external acceptance project and hymx limited to test-only changes.

**Architecture:** Keep `encrypttags` responsible for realistic external VM behavior and helper-level leakage detection. Add narrow hymx unit tests for deterministic edge cases that are difficult to exercise through a live node. Do not edit hymx production files.

**Tech Stack:** Go 1.24, `testing`, `stretchr/testify/require`, `net/http/httptest`, `github.com/hymatrix/hymx/utils/tagcrypto`, `github.com/permadao/goar`, `github.com/everFinance/goether`.

---

## File Structure

Modify these `encrypttags` files:

- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/echo/echo_test.go`
  - Adds VM unit coverage for missing encrypted params and checkpoint secret exclusion.
- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`
  - Adds a small success-summary helper so e2e output can be tested for redaction.
- `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags_test.go`
  - Adds helper tests for leakage detection, output conversion, and success-summary redaction.

Modify these hymx test files only:

- `/Users/webbergao/work/src/HymxWorkspace/hymx/utils/tagcrypto/tagcrypto_test.go`
  - Adds tagcrypto malformed input and mixed tag tests.
- `/Users/webbergao/work/src/HymxWorkspace/hymx/sdk/encrypted_tags_test.go`
  - Adds SDK metadata and network-boundary tests.
- `/Users/webbergao/work/src/HymxWorkspace/hymx/node/encrypted_tags_test.go`
  - Adds node decryption and checkpoint failure-path tests.

Do not modify hymx non-test files.

---

### Task 1: Expand Encrypttags Echo VM Unit Tests

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/echo/echo_test.go`

- [ ] **Step 1: Add failing test for missing encrypted params**

Append this test to `echo/echo_test.go`:

```go
func TestEchoApplyHandlesMissingEncryptedParams(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{},
		},
	})
	require.NoError(t, err)

	res := vm.Apply("sender", vmmSchema.Meta{
		Params: map[string]string{
			"Plain": "plain-e2e",
		},
	})
	require.NoError(t, res.Error)

	output, ok := res.Output.(map[string]string)
	require.True(t, ok)
	require.Empty(t, output["SpawnSecret"])
	require.Empty(t, output["Secret"])
	require.Equal(t, "plain-e2e", output["Plain"])
}
```

- [ ] **Step 2: Run the focused encrypttags echo tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./echo -run 'TestEchoApplyHandlesMissingEncryptedParams|TestEchoApplyOutputsDecryptedParams|TestEchoCheckpointDoesNotStoreSecrets' -count=1
```

Expected: PASS. If it fails, inspect the failure; the current echo VM should already support this behavior through map zero values.

- [ ] **Step 3: Add failing test that checkpoint excludes all sentinel secrets**

Append this test to `echo/echo_test.go`:

```go
func TestEchoCheckpointExcludesKnownSecretSentinels(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{
				"SpawnSecret": "spawn-secret-e2e",
				"Secret":      "message-secret-e2e",
				"Plain":       "plain-e2e",
			},
		},
	})
	require.NoError(t, err)

	checkpoint, err := vm.Checkpoint()
	require.NoError(t, err)
	require.NotContains(t, checkpoint, "spawn-secret-e2e")
	require.NotContains(t, checkpoint, "message-secret-e2e")
	require.NotContains(t, checkpoint, "plain-e2e")
}
```

- [ ] **Step 4: Run the full encrypttags echo package**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./echo -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add echo/echo_test.go
git commit -m "test: expand echo encrypted tag coverage"
```

---

### Task 2: Add Encrypttags Helper and Output Redaction Tests

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags.go`
- Modify: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/examples/encrypttags_test.go`

- [ ] **Step 1: Add failing tests for helper edge cases**

Append these tests to `examples/encrypttags_test.go`:

```go
func TestEncryptedTagStoredFlagsLeakFromUnrelatedTag(t *testing.T) {
	encrypted, leaked := encryptedTagStored([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: tagcrypto.CipherValuePrefix + ":" + tagcrypto.KeyTypeEthereumECIES + ":ciphertext"},
		{Name: "Debug", Value: "private-value"},
	}, tagcrypto.EncryptedTagPrefix+"Secret", "private-value", tagcrypto.KeyTypeEthereumECIES)

	require.True(t, encrypted)
	require.True(t, leaked)
}

func TestEncryptedTagStoredRejectsWrongKeyTypePrefix(t *testing.T) {
	encrypted, leaked := encryptedTagStored([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: tagcrypto.CipherValuePrefix + ":" + tagcrypto.KeyTypeArweaveRSAOAEP + ":ciphertext"},
	}, tagcrypto.EncryptedTagPrefix+"Secret", "private-value", tagcrypto.KeyTypeEthereumECIES)

	require.False(t, encrypted)
	require.False(t, leaked)
}

func TestEncryptedTagStoredRejectsMissingEncryptedTag(t *testing.T) {
	encrypted, leaked := encryptedTagStored([]goarSchema.Tag{
		{Name: "Plain", Value: "public-value"},
	}, tagcrypto.EncryptedTagPrefix+"Secret", "private-value", tagcrypto.KeyTypeEthereumECIES)

	require.False(t, encrypted)
	require.False(t, leaked)
}

func TestOutputMapAcceptsStringMap(t *testing.T) {
	output, err := outputMap(map[string]interface{}{
		"SpawnSecret": "spawn-secret-e2e",
		"Secret":      "message-secret-e2e",
		"Plain":       "plain-e2e",
	})

	require.NoError(t, err)
	require.Equal(t, "spawn-secret-e2e", output["SpawnSecret"])
	require.Equal(t, "message-secret-e2e", output["Secret"])
	require.Equal(t, "plain-e2e", output["Plain"])
}

func TestOutputMapRejectsNonMap(t *testing.T) {
	_, err := outputMap("not-a-map")

	require.Error(t, err)
}
```

- [ ] **Step 2: Run helper tests and confirm current behavior**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run 'TestEncryptedTagStored|TestOutputMap' -count=1
```

Expected: PASS for helper tests. If any fail, fix `encryptedTagStored` or `outputMap` inside `examples/encrypttags.go` only.

- [ ] **Step 3: Add success-summary redaction helper test**

Add imports to `examples/encrypttags_test.go`:

```go
import (
	"bytes"
	"testing"

	"github.com/hymatrix/hymx/utils/tagcrypto"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)
```

Replace the existing single-import block with the block above, then append this test:

```go
func TestPrintEncryptedTagsSuccessRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	printEncryptedTagsSuccess(&buf, encryptedTagsSummary{
		SpawnEncrypted:   true,
		SpawnLeaked:      false,
		MessageEncrypted: true,
		MessageLeaked:    false,
		ReservedRejected: true,
		Plain:            "plain-e2e",
	})

	output := buf.String()
	require.Contains(t, output, "E2E encrypted tags passed")
	require.Contains(t, output, "RAW spawn encrypted=true plaintext_leaked=false")
	require.Contains(t, output, "RAW message encrypted=true plaintext_leaked=false")
	require.Contains(t, output, "RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=plain-e2e")
	require.Contains(t, output, "reserved encrypted tag rejected=true")
	require.NotContains(t, output, "spawn-secret-e2e")
	require.NotContains(t, output, "message-secret-e2e")
}
```

- [ ] **Step 4: Run the redaction test and verify it fails**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./examples -run TestPrintEncryptedTagsSuccessRedactsSecrets -count=1
```

Expected: FAIL with `undefined: printEncryptedTagsSuccess` and `undefined: encryptedTagsSummary`.

- [ ] **Step 5: Implement the success-summary helper**

In `examples/encrypttags.go`, add `io` and `os` to the imports:

```go
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)
```

Add this type and helper near the bottom of the file, above `sendReservedEncryptedTagToNode`:

```go
type encryptedTagsSummary struct {
	SpawnEncrypted   bool
	SpawnLeaked      bool
	MessageEncrypted bool
	MessageLeaked    bool
	ReservedRejected bool
	Plain            string
}

func printEncryptedTagsSuccess(w io.Writer, summary encryptedTagsSummary) {
	fmt.Fprintln(w, "E2E encrypted tags passed")
	fmt.Fprintf(w, "RAW spawn encrypted=%v plaintext_leaked=%v\n", summary.SpawnEncrypted, summary.SpawnLeaked)
	fmt.Fprintf(w, "RAW message encrypted=%v plaintext_leaked=%v\n", summary.MessageEncrypted, summary.MessageLeaked)
	fmt.Fprintf(w, "RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=%s\n", summary.Plain)
	fmt.Fprintf(w, "reserved encrypted tag rejected=%v\n", summary.ReservedRejected)
}
```

Replace the final five `fmt.Println` / `fmt.Printf` success lines in `encryptTagsCmd` with:

```go
	printEncryptedTagsSuccess(os.Stdout, encryptedTagsSummary{
		SpawnEncrypted:   spawnEncrypted,
		SpawnLeaked:      spawnLeaked,
		MessageEncrypted: messageEncrypted,
		MessageLeaked:    messageLeaked,
		ReservedRejected: reservedRejected,
		Plain:            output["Plain"],
	})
	return nil
```

Also remove these earlier plaintext debug prints from `encryptTagsCmd`:

```go
	fmt.Printf("re SpawnSecret: %s\n", output["SpawnSecret"])
	fmt.Printf("re Secret: %s\n", output["Secret"])
	fmt.Printf("re Plain: %s\n", output["Plain"])
```

The plain value is still emitted by `printEncryptedTagsSuccess` as `Plain=plain-e2e`; `SpawnSecret` and `Secret` must not be printed directly.

- [ ] **Step 6: Run gofmt and examples tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
gofmt -w examples/encrypttags.go examples/encrypttags_test.go
go test ./examples -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 2**

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
git add examples/encrypttags.go examples/encrypttags_test.go
git commit -m "test: cover encrypttags helper safety checks"
```

---

### Task 3: Add Hymx tagcrypto Negative and Mixed-Tag Tests

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/hymx/utils/tagcrypto/tagcrypto_test.go`

- [ ] **Step 1: Add tests for name validation and non-encrypted passthrough**

Append these tests to `utils/tagcrypto/tagcrypto_test.go`:

```go
func TestInvalidEncryptedTagNameRejected(t *testing.T) {
	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix, Value: "private-value"}}

	err := ValidateEncryptedTagNames(tags)

	require.Error(t, err)
}

func TestEncryptTagsLeavesPlainTagsUnchanged(t *testing.T) {
	tags := []goarSchema.Tag{{Name: "Plain", Value: "public-value"}}

	encrypted, changed, err := EncryptTags(tags, "", KeyTypeEthereumECIES)

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, tags, encrypted)
}
```

- [ ] **Step 2: Run the focused tagcrypto tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
go test ./utils/tagcrypto -run 'TestInvalidEncryptedTagNameRejected|TestEncryptTagsLeavesPlainTagsUnchanged' -count=1
```

Expected: PASS.

- [ ] **Step 3: Add tests for mixed plain/encrypted tags and encrypted name extraction**

Append these tests:

```go
func TestMixedPlainAndEncryptedTagsRoundTrip(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{
		{Name: "Plain", Value: "public-value"},
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "public-value", encrypted[0].Value)
	require.NotContains(t, encrypted[1].Value, "private-value")

	names, err := EncryptedPlainTagNames(encrypted)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"Secret": true}, names)

	decrypted, changed, err := DecryptTags(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []goarSchema.Tag{
		{Name: "Plain", Value: "public-value"},
		{Name: "Secret", Value: "private-value"},
	}, decrypted)
}
```

- [ ] **Step 4: Add tests for malformed cipher and wrong signer/key type**

Append these tests:

```go
func TestDecryptTagsRejectsMalformedCipherValue(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)

	_, _, err = DecryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
	}, nodeSigner)

	require.Error(t, err)
}

func TestEncryptTagsRejectsUnsupportedKeyType(t *testing.T) {
	_, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, "public-key", "unsupported-key-type")

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported encryption key type")
}

func TestDecryptTagsRejectsUnsupportedCipherKeyType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)

	_, _, err = DecryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: CipherValuePrefix + ":unsupported-key-type:Y2lwaGVy"},
	}, nodeSigner)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported encryption key type")
}

func TestDecryptTagsRejectsWrongSignerType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	arweaveSigner, err := goar.NewSignerFromPath("../../examples/test_keyfile.json")
	require.NoError(t, err)

	encrypted, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)

	_, _, err = DecryptTags(encrypted, arweaveSigner)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ethereum encrypted tag requires ethereum signer")
}
```

- [ ] **Step 5: Add reserved-name consistency test**

Append this test:

```go
func TestEncryptedReservedNameRejectedConsistently(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Type", Value: "Message"}}

	require.Error(t, ValidateEncryptedTagNames(tags))
	_, err = EncryptedPlainTagNames(tags)
	require.Error(t, err)
	_, _, err = EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.Error(t, err)
}
```

- [ ] **Step 6: Run full tagcrypto tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
gofmt -w utils/tagcrypto/tagcrypto_test.go
go test ./utils/tagcrypto -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 3**

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
git add utils/tagcrypto/tagcrypto_test.go
git commit -m "test: cover encrypted tagcrypto edge cases"
```

---

### Task 4: Add Hymx SDK Metadata and Network-Boundary Tests

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/hymx/sdk/encrypted_tags_test.go`

- [ ] **Step 1: Add helper for user SDK construction**

Add this helper near the existing helper functions in `sdk/encrypted_tags_test.go`:

```go
func newEncryptedTagTestSDK(t *testing.T, baseURL string) *SDK {
	t.Helper()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	return NewFromBundler(baseURL, userBundler)
}
```

- [ ] **Step 2: Add failing tests for missing encryption metadata**

Append these tests:

```go
func TestSendEncryptedTagRequiresInfoPublicKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionKeyType: tagcrypto.KeyTypeEthereumECIES,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "does not advertise encryption metadata")
}

func TestSendEncryptedTagRequiresInfoKeyType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err = s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "does not advertise encryption metadata")
}
```

- [ ] **Step 3: Add test that plain send does not fetch `/info`**

Append this test:

```go
func TestSendPlainTagsDoesNotFetchInfo(t *testing.T) {
	infoRequests := 0
	var submitted goarSchema.BundleItem
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			infoRequests++
			http.Error(w, "info should not be fetched", http.StatusInternalServerError)
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			submitted = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("process-id", "payload", []goarSchema.Tag{{Name: "Plain", Value: "public-value"}})

	require.NoError(t, err)
	require.Zero(t, infoRequests)
	require.Equal(t, "public-value", tagValue(submitted.Tags, "Plain"))
}
```

- [ ] **Step 4: Add redirect test for unusable redirected nodes**

Append this test:

```go
func TestSendEncryptedRedirectReturnsErrorWhenRedirectInfoIsUnusable(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	badRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer badRedirect.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: badRedirect.URL}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	s := newEncryptedTagTestSDK(t, primary.URL)

	res, redirectedURL, err := s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Nil(t, res)
	require.Empty(t, redirectedURL)
}
```

- [ ] **Step 5: Add test that encrypted protocol tag avoids network**

Append this test:

```go
func TestSendRejectsEncryptedProtocolTagBeforeNetworkAccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "network should not be reached", http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("", "", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Type", Value: schema.TypeMessage}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
	require.Zero(t, requests)
}
```

- [ ] **Step 6: Run focused SDK tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
gofmt -w sdk/encrypted_tags_test.go
go test ./sdk -run 'TestSendEncryptedTagRequiresInfo|TestSendPlainTagsDoesNotFetchInfo|TestSendEncryptedRedirectReturnsErrorWhenRedirectInfoIsUnusable|TestSendRejectsEncryptedProtocolTagBeforeNetworkAccess' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
git add sdk/encrypted_tags_test.go
git commit -m "test: cover encrypted SDK metadata failures"
```

---

### Task 5: Add Hymx Node Decryption and Checkpoint Failure Tests

**Files:**
- Modify: `/Users/webbergao/work/src/HymxWorkspace/hymx/node/encrypted_tags_test.go`

- [ ] **Step 1: Add malformed encrypted message decrypt test**

Append this test to `node/encrypted_tags_test.go`:

```go
func TestDecryptInternalItemRejectsMalformedEncryptedValue(t *testing.T) {
	rawTags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
	}

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "target-process", "", rawTags)
	require.NoError(t, err)

	n := &Node{signer: userSigner}
	internalItem, instance, err := n.decryptInternalItem(rawItem)

	require.Error(t, err)
	require.Empty(t, internalItem.Id)
	require.Nil(t, instance)
}
```

- [ ] **Step 2: Add wrong signer type decrypt test**

Append this test:

```go
func TestDecryptInternalItemRejectsWrongSignerType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	encryptedTags, _, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		nodeBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
	require.NoError(t, err)
	rawTags := utils.MergeTags([]goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
	}, encryptedTags)

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "target-process", "", rawTags)
	require.NoError(t, err)
	arweaveSigner, err := goar.NewSignerFromPath("../examples/test_keyfile.json")
	require.NoError(t, err)

	n := &Node{signer: arweaveSigner}
	_, _, err = n.decryptInternalItem(rawItem)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ethereum encrypted tag requires ethereum signer")
}
```

- [ ] **Step 3: Add sanitize checkpoint wrong raw type test**

Append this test:

```go
func TestSanitizeCheckpointSnapshotRejectsNonProcessRawItem(t *testing.T) {
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawTags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
	}
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "target-process", "", rawTags)
	require.NoError(t, err)

	n := &Node{db: checkpointTestDB{items: map[string]goarSchema.BundleItem{"process-id": rawItem}}}
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "process-id",
			},
		},
	}

	_, err = n.sanitizeCheckpointSnapshot(snap)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid type")
}
```

- [ ] **Step 4: Add decrypt snapshot malformed tag test**

Append this test:

```go
func TestDecryptSnapshotEnvRejectsMalformedEncryptedTag(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	n := &Node{signer: nodeSigner}
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Process: hymxSchema.Process{
				Tags: []goarSchema.Tag{
					{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
				},
			},
			Meta: vmmSchema.Meta{
				Params: map[string]string{"Plain": "public-value"},
			},
		},
	}

	restored, err := n.decryptSnapshotEnv(snap)

	require.Error(t, err)
	require.Equal(t, snap.Env.Meta.Params, restored.Env.Meta.Params)
	require.Equal(t, snap.Env.Process.Tags, restored.Env.Process.Tags)
}
```

- [ ] **Step 5: Run focused node tests**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
gofmt -w node/encrypted_tags_test.go
go test ./node -run 'TestDecryptInternalItemRejectsMalformedEncryptedValue|TestDecryptInternalItemRejectsWrongSignerType|TestSanitizeCheckpointSnapshotRejectsNonProcessRawItem|TestDecryptSnapshotEnvRejectsMalformedEncryptedTag' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
git add node/encrypted_tags_test.go
git commit -m "test: cover encrypted node failure paths"
```

---

### Task 6: Final Verification

**Files:**
- Read: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/docs/superpowers/specs/2026-04-29-encrypttags-test-coverage-design.md`
- Read: `/Users/webbergao/work/src/HymxWorkspace/encrypttags/docs/superpowers/plans/2026-04-29-encrypttags-test-coverage.md`

- [ ] **Step 1: Verify no hymx production files changed**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
git diff --name-only HEAD~3..HEAD
```

Expected: only these paths appear:

```text
utils/tagcrypto/tagcrypto_test.go
sdk/encrypted_tags_test.go
node/encrypted_tags_test.go
```

- [ ] **Step 2: Run encrypttags validation**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go test ./...
go build -o ./build/hymx-node ./cmd
```

Expected: both commands exit 0.

- [ ] **Step 3: Run focused hymx validation**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
go test ./utils/tagcrypto ./sdk ./node ./vmm/...
```

Expected: command exits 0.

- [ ] **Step 4: Run full hymx validation if time allows**

Run:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/hymx
go test ./...
```

Expected: command exits 0. If it fails outside the changed packages, record the failing package and error in the final report.

- [ ] **Step 5: Report live-node e2e status**

Do not run this unless Redis is available and a hymx node can be started:

```bash
cd /Users/webbergao/work/src/HymxWorkspace/encrypttags
go run ./cmd -c ./cmd/config.yaml
go run ./examples init
go run ./examples encrypttags
```

Expected success output includes:

```text
E2E encrypted tags passed
RAW spawn encrypted=true plaintext_leaked=false
RAW message encrypted=true plaintext_leaked=false
RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=plain-e2e
reserved encrypted tag rejected=true
```

- [ ] **Step 6: Commit final verification note only if files changed**

If verification required editing docs or tests, commit those edits in the relevant repository. If no files changed, do not create an empty commit.

---

## Self-Review Notes

- Spec coverage: Encrypttags echo/helper/manual e2e requirements map to Tasks 1 and 2. Hymx tagcrypto, SDK, and node negative-input requirements map to Tasks 3, 4, and 5. Validation requirements map to Task 6.
- Placeholder scan: No task contains `TBD`, `TODO`, or an unspecified implementation step.
- Scope check: The plan intentionally touches two independent git repositories. Each task is repo-scoped, and no task moves files between repositories.
- Type consistency: Test snippets use existing hymx/encrypttags types: `goarSchema.Tag`, `nodeSchema.Info`, `registrySchema.Node`, `vmmSchema.Snapshot`, and existing helper names from current tests.
