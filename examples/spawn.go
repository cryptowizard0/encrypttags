package main

import (
	"fmt"
	"io"
	"os"
)

func printSpawnEchoSuccess(w io.Writer, processID string) {
	fmt.Fprintln(w, "SPAWN_ECHO passed")
	fmt.Fprintf(w, "PROCESS pid=%s\n", processID)
}

func spawnEchoCmd() error {
	info, err := s.Client.Info()
	if err != nil {
		return fmt.Errorf("read node info: %w", err)
	}
	if info.Token == "" || info.Registry == "" {
		return fmt.Errorf("core token/registry not initialized; run `go run ./examples init` first")
	}

	spawnRes, err := s.SpawnAndWait(echoModule, s.GetAddress(), nil)
	if err != nil {
		return fmt.Errorf("spawn echo: %w", err)
	}
	printSpawnEchoSuccess(os.Stdout, spawnRes.Id)
	return nil
}
