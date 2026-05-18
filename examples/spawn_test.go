package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintSpawnEchoSuccess(t *testing.T) {
	var buf bytes.Buffer

	printSpawnEchoSuccess(&buf, "pid-1")

	output := buf.String()
	require.Contains(t, output, "SPAWN_ECHO passed")
	require.Contains(t, output, "PROCESS pid=pid-1")
}
