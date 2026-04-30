package main

import (
	"bytes"
	"testing"

	"github.com/hymatrix/hymx/utils/tagcrypto"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestEncryptedTagStoredDetectsCipherAndLeakage(t *testing.T) {
	encrypted, leaked := encryptedTagStored([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: tagcrypto.CipherValuePrefix + ":" + tagcrypto.KeyTypeEthereumECIES + ":ciphertext"},
		{Name: "Plain", Value: "public-value"},
	}, tagcrypto.EncryptedTagPrefix+"Secret", "private-value", tagcrypto.KeyTypeEthereumECIES)

	require.True(t, encrypted)
	require.False(t, leaked)
}

func TestEncryptedTagStoredFlagsPlaintextLeak(t *testing.T) {
	encrypted, leaked := encryptedTagStored([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, tagcrypto.EncryptedTagPrefix+"Secret", "private-value", tagcrypto.KeyTypeEthereumECIES)

	require.False(t, encrypted)
	require.True(t, leaked)
}

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

func TestOutputMapRejectsUnexpectedShape(t *testing.T) {
	_, err := outputMap(map[string]interface{}{
		"Secret": 42,
	})

	require.Error(t, err)
}

func TestPrintEncryptedTagsSuccessRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	printEncryptedTagsSuccess(&buf, encryptedTagsSummary{
		SpawnEncrypted:   true,
		SpawnLeaked:      false,
		MessageEncrypted: true,
		MessageLeaked:    false,
		ReservedRejected: true,
		ProcessID:        "process-id",
		Plain:            "plain-e2e",
	})

	output := buf.String()
	require.Contains(t, output, "E2E encrypted tags passed")
	require.Contains(t, output, "RAW spawn encrypted=true plaintext_leaked=false")
	require.Contains(t, output, "RAW message encrypted=true plaintext_leaked=false")
	require.Contains(t, output, "PROCESS pid=process-id")
	require.Contains(t, output, "RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=plain-e2e")
	require.Contains(t, output, "reserved encrypted tag rejected=true")
	require.NotContains(t, output, "spawn-secret-e2e")
	require.NotContains(t, output, "message-secret-e2e")
}
