package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncrypttagsE2EScriptContract(t *testing.T) {
	scriptPath := filepath.Join("encrypttags-e2e.sh")

	info, err := os.Stat(scriptPath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.NotZero(t, info.Mode()&0111)

	cmd := exec.Command("bash", "-n", scriptPath)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	cmd = exec.Command("bash", scriptPath, "--help")
	cmd.Dir = "."
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "Usage:")

	cmd = exec.Command("bash", scriptPath, "--dry-run")
	cmd.Dir = "."
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "docker rm -f")
	require.Contains(t, string(output), "go build -o")
	require.Contains(t, string(output), "/cmd && ../build/hymx-node")
	require.Contains(t, string(output), "checkpoint-restore")
	require.Contains(t, string(output), "STEP 1/7")
	require.Contains(t, string(output), "STEP 7/7")
	require.NotContains(t, string(output), "stop-resume")
	require.Contains(t, string(output), "RESULT")
}

func TestStopResumeE2EScriptContract(t *testing.T) {
	scriptPath := filepath.Join("stop-resume-e2e.sh")

	info, err := os.Stat(scriptPath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.NotZero(t, info.Mode()&0111)

	cmd := exec.Command("bash", "-n", scriptPath)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	cmd = exec.Command("bash", scriptPath, "--help")
	cmd.Dir = "."
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "Usage:")

	cmd = exec.Command("bash", scriptPath, "--dry-run")
	cmd.Dir = "."
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "docker rm -f")
	require.Contains(t, string(output), "go build -o")
	require.Contains(t, string(output), "/cmd && ../build/hymx-node")
	require.Contains(t, string(output), "stop-resume")
	require.Contains(t, string(output), "spawn-echo")
	require.Contains(t, string(output), "STEP 1/5")
	require.Contains(t, string(output), "STEP 5/5")
	require.Contains(t, string(output), ":8081")
	require.NotContains(t, string(output), "go run ./examples encrypttags")
	require.NotContains(t, string(output), "checkpoint-restore")
	require.Contains(t, string(output), "RESULT")
}
