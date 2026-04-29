package main

import (
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

func TestOutputMapRejectsUnexpectedShape(t *testing.T) {
	_, err := outputMap(map[string]interface{}{
		"Secret": 42,
	})

	require.Error(t, err)
}
