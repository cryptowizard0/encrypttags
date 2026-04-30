package main

import (
	"bytes"
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

func TestCheckpointRestoreCmdRequiresPid(t *testing.T) {
	var buf bytes.Buffer

	err := checkpointRestoreCmd(&buf, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
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

func echoResultMessageForTest(t *testing.T, output map[string]string) string {
	t.Helper()
	by, err := json.Marshal(vmmSchema.VmmResult{Output: output})
	require.NoError(t, err)
	return string(by)
}
