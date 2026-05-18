package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	hymxSchema "github.com/hymatrix/hymx/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/require"
)

const checkpointTestPid = "process-id"

func TestVerifyEncryptedCheckpointAcceptsEncryptedSecretTag(t *testing.T) {
	cipherValue := checkpointCipherValue()
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: cipherValue},
	}, map[string]string{
		vmmSchema.EncryptedTagPrefix + "Secret": cipherValue,
		"Plain":                                 "plain-e2e",
	})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.NoError(t, err)
	require.True(t, result.Encrypted)
	require.False(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsPlainMessageSecretParam(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, map[string]string{"Secret": e2eMessageSecret})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsMessageSecretLeakFromRawJSON(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, map[string]string{"Debug": e2eMessageSecret})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsMessageSecretLeakInParams(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, map[string]string{"Debug": e2eMessageSecret})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsPlainMessageSecretTag(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: "Secret", Value: e2eMessageSecret},
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.Error(t, err)
	require.True(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointAcceptsMissingEncryptedSecretTag(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: "Plain", Value: "plain-e2e"},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.NoError(t, err)
	require.False(t, result.Encrypted)
	require.False(t, result.PlaintextLeaked)
}

func TestVerifyEncryptedCheckpointRejectsMalformedCiphertext(t *testing.T) {
	snap := checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: "not base64"},
	}, map[string]string{})
	raw := checkpointSnapshotJSON(t, snap)

	result, err := verifyEncryptedCheckpoint(snap, raw, checkpointTestPid)

	require.Error(t, err)
	require.False(t, result.Encrypted)
}

func TestFindCheckpointForProcessSelectsMatchingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-other.json", checkpointItemForTest(t, checkpointSnapshotForTest("other-process", []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, nil)))
	expectedPath := writeCheckpointFileForTest(t, dir, "ckp-process.json", checkpointItemForTest(t, checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, nil)))

	found, err := findCheckpointForProcess(dir, checkpointTestPid)

	require.NoError(t, err)
	require.Equal(t, expectedPath, found.Path)
	require.Equal(t, checkpointTestPid, found.Snapshot.Env.Meta.Pid)
}

func TestFindCheckpointForProcessRejectsMismatchedProcess(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-other.json", checkpointItemForTest(t, checkpointSnapshotForTest("other-process", []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, nil)))

	_, err := findCheckpointForProcess(dir, checkpointTestPid)

	require.Error(t, err)
	require.Contains(t, err.Error(), "checkpoint not found")
}

func TestFindCheckpointForProcessInDirsSearchesMultipleDirs(t *testing.T) {
	root := t.TempDir()
	emptyDir := filepath.Join(root, "examples", "ckp")
	cmdDir := filepath.Join(root, "cmd", "ckp")
	require.NoError(t, os.MkdirAll(emptyDir, 0755))
	require.NoError(t, os.MkdirAll(cmdDir, 0755))
	expectedPath := writeCheckpointFileForTest(t, cmdDir, "ckp-process.json", checkpointItemForTest(t, checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, nil)))

	found, err := findCheckpointForProcessInDirs([]string{emptyDir, cmdDir}, checkpointTestPid)

	require.NoError(t, err)
	require.Equal(t, expectedPath, found.Path)
}

func TestFindCheckpointForProcessInDirsExplainsShutdownRequirement(t *testing.T) {
	dir := t.TempDir()

	_, err := findCheckpointForProcessInDirs([]string{dir}, checkpointTestPid)

	require.Error(t, err)
	require.Contains(t, err.Error(), "stop the node")
}

func TestLoadCheckpointItemRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ckp-bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0644))

	_, err := loadCheckpointItem(path)

	require.Error(t, err)
}

func TestCheckpointCmdPrintsEncryptedStatus(t *testing.T) {
	dir := t.TempDir()
	writeCheckpointFileForTest(t, dir, "ckp-process.json", checkpointItemForTest(t, checkpointSnapshotForTest(checkpointTestPid, []goarSchema.Tag{
		{Name: vmmSchema.EncryptedTagPrefix + "Secret", Value: checkpointCipherValue()},
	}, nil)))
	t.Setenv("ENCRYPTTAGS_CKP_DIR", dir)
	var buf bytes.Buffer

	err := checkpointCmd(&buf, []string{checkpointTestPid})

	require.NoError(t, err)
	require.Contains(t, buf.String(), "CHECKPOINT plaintext_leaked=false")
}

func TestCheckpointCmdRequiresPid(t *testing.T) {
	var buf bytes.Buffer

	err := checkpointCmd(&buf, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}

func TestVerifyEchoMessageAcceptsExpectedOutput(t *testing.T) {
	message := echoResultMessageForTest(t, map[string]string{
		"Secret": "custom-secret",
		"Plain":  "custom-plain",
	})

	output, err := verifyEchoMessage(message, echoExpectation{
		Secret: "custom-secret",
		Plain:  "custom-plain",
	})

	require.NoError(t, err)
	require.Equal(t, "custom-secret", output["Secret"])
	require.Equal(t, "custom-plain", output["Plain"])
}

func TestVerifyEchoMessageRejectsUnexpectedOutput(t *testing.T) {
	message := echoResultMessageForTest(t, map[string]string{
		"Secret": "missing-after-restore",
		"Plain":  e2ePlain,
	})

	_, err := verifyEchoMessage(message, echoExpectation{
		Secret: e2eMessageSecret,
		Plain:  e2ePlain,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected echo output")
}

func TestVerifyEchoMessageReportsVMMResultError(t *testing.T) {
	by, err := json.Marshal(vmmSchema.VmmResult{Error: "err_invalid_nonce"})
	require.NoError(t, err)

	_, err = verifyEchoMessage(string(by), echoExpectation{
		Secret: e2eMessageSecret,
		Plain:  e2ePlain,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "vmm result error")
	require.Contains(t, err.Error(), "err_invalid_nonce")
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

func checkpointCipherValue() string {
	return base64.StdEncoding.EncodeToString([]byte("ciphertext"))
}

func echoResultMessageForTest(t *testing.T, output map[string]string) string {
	t.Helper()
	by, err := json.Marshal(vmmSchema.VmmResult{Output: output})
	require.NoError(t, err)
	return string(by)
}
