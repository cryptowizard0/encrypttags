package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

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

func checkpointDirs() []string {
	if dir := os.Getenv("ENCRYPTTAGS_CKP_DIR"); dir != "" {
		return []string{dir}
	}
	candidates := []string{
		"./ckp",
		"../ckp",
		"cmd/ckp",
		"../cmd/ckp",
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Dir(filepath.Dir(file))
		candidates = append(candidates,
			filepath.Join(repoRoot, "ckp"),
			filepath.Join(repoRoot, "cmd", "ckp"),
		)
	}
	return uniqueCheckpointDirs(candidates)
}

func uniqueCheckpointDirs(candidates []string) []string {
	dirs := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		dir := filepath.Clean(candidate)
		key := dir
		if abs, err := filepath.Abs(dir); err == nil {
			key = abs
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dirs = append(dirs, dir)
	}
	return dirs
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

func findCheckpointForProcessInDirs(dirs []string, processID string) (checkpointMatch, error) {
	var best checkpointMatch
	searched := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		searched = append(searched, dir)
		match, err := findCheckpointForProcess(dir, processID)
		if err != nil {
			if strings.Contains(err.Error(), "checkpoint not found") {
				continue
			}
			return checkpointMatch{}, err
		}
		if best.Path == "" || match.modTime.After(best.modTime) {
			best = match
		}
	}
	if best.Path == "" {
		return checkpointMatch{}, fmt.Errorf("checkpoint not found for process %s in %s; stop the node first so hymx writes ckp/ckp-*.json, then rerun checkpoint or set ENCRYPTTAGS_CKP_DIR", processID, strings.Join(searched, ", "))
	}
	return best, nil
}

func verifyEncryptedCheckpoint(snapshot vmmSchema.Snapshot, rawSnapshotJSON []byte, processID string) (checkpointVerification, error) {
	result := checkpointVerification{}
	if snapshot.Env.Meta.Pid != processID {
		return result, fmt.Errorf("checkpoint process mismatch: got %s want %s", snapshot.Env.Meta.Pid, processID)
	}
	rawSnapshot := string(rawSnapshotJSON)
	if strings.Contains(rawSnapshot, e2eMessageSecret) {
		result.PlaintextLeaked = true
		return result, fmt.Errorf("checkpoint plaintext secret leaked")
	}

	if snapshot.Env.Meta.Params["Secret"] == e2eMessageSecret {
		result.PlaintextLeaked = true
		return result, fmt.Errorf("checkpoint meta params contain plaintext Secret")
	}
	for _, tag := range snapshot.Env.Process.Tags {
		if tag.Name == "Secret" && tag.Value == e2eMessageSecret {
			result.PlaintextLeaked = true
			return result, fmt.Errorf("checkpoint process tags contain plaintext Secret")
		}
		if tag.Name != vmmSchema.EncryptedTagPrefix+"Secret" {
			continue
		}
		if _, err := base64.StdEncoding.DecodeString(tag.Value); err != nil || tag.Value == "" {
			return result, fmt.Errorf("encrypted Secret is not valid base64 ciphertext")
		}
		result.Encrypted = true
	}
	return result, nil
}

func checkpointCmd(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: checkpoint <pid>")
	}
	match, err := findCheckpointForProcessInDirs(checkpointDirs(), args[0])
	if err != nil {
		return err
	}
	result, err := verifyEncryptedCheckpoint(match.Snapshot, match.RawSnapshotJSON, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "CHECKPOINT plaintext_leaked=%v\n", result.PlaintextLeaked)
	return nil
}

func checkpointRestoreCmd(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: checkpoint-restore <pid>")
	}
	msgRes, err := s.SendMessageWithEncryptedParamsAndWait(
		args[0],
		"",
		[]goarSchema.Tag{{Name: "Plain", Value: e2ePlain}},
		[]goarSchema.Tag{{Name: "Secret", Value: e2eMessageSecret}},
	)
	if err != nil {
		return fmt.Errorf("send restore check message: %w", err)
	}
	if _, err := verifyEchoMessage(msgRes.Message, echoExpectation{
		Secret: e2eMessageSecret,
		Plain:  e2ePlain,
	}); err != nil {
		return err
	}
	fmt.Fprintln(w, "CHECKPOINT restore_decrypted=true")
	return nil
}
