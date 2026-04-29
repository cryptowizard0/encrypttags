package echo

import (
	"encoding/json"
	"testing"

	"github.com/cryptowizard0/encrypttags/echo/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/require"
)

func TestEchoApplyOutputsDecryptedParams(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{
				"SpawnSecret": "spawn-secret-e2e",
			},
		},
	})
	require.NoError(t, err)

	res := vm.Apply("sender", vmmSchema.Meta{
		Params: map[string]string{
			"Secret": "message-secret-e2e",
			"Plain":  "plain-e2e",
		},
	})
	require.NoError(t, res.Error)

	output, ok := res.Output.(map[string]string)
	require.True(t, ok)
	require.Equal(t, "spawn-secret-e2e", output["SpawnSecret"])
	require.Equal(t, "message-secret-e2e", output["Secret"])
	require.Equal(t, "plain-e2e", output["Plain"])
	require.Empty(t, res.Cache)
}

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

func TestEchoCheckpointDoesNotStoreSecrets(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{
				"SpawnSecret": "spawn-secret-e2e",
			},
		},
	})
	require.NoError(t, err)

	checkpoint, err := vm.Checkpoint()
	require.NoError(t, err)
	require.NotContains(t, checkpoint, "spawn-secret-e2e")

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(checkpoint), &payload))
	require.Equal(t, schema.ModuleFormat, payload["Module-Format"])
}

func TestEchoCheckpointExcludesKnownSecretSentinels(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{
				"SpawnSecret": "spawn-secret-e2e",
			},
		},
	})
	require.NoError(t, err)

	res := vm.Apply("sender", vmmSchema.Meta{
		Params: map[string]string{
			"Secret": "message-secret-e2e",
			"Plain":  "plain-e2e",
		},
	})
	require.NoError(t, res.Error)

	checkpoint, err := vm.Checkpoint()
	require.NoError(t, err)
	require.NotContains(t, checkpoint, "spawn-secret-e2e")
	require.NotContains(t, checkpoint, "message-secret-e2e")
	require.NotContains(t, checkpoint, "plain-e2e")
}

func TestEchoRestoreKeepsSpawnSecretFromRecoveredEnv(t *testing.T) {
	vm, err := New(vmmSchema.Env{
		Meta: vmmSchema.Meta{
			Params: map[string]string{
				"SpawnSecret": "spawn-secret-e2e",
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, vm.Restore(`{"Module-Format":"hymx.e2e.echo.0.0.0"}`))

	res := vm.Apply("sender", vmmSchema.Meta{
		Params: map[string]string{
			"Secret": "message-secret-e2e",
			"Plain":  "plain-e2e",
		},
	})
	output := res.Output.(map[string]string)
	require.Equal(t, "spawn-secret-e2e", output["SpawnSecret"])
}
