package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type encryptedTagsSummary struct {
	MessageEncrypted bool
	MessageLeaked    bool
	ProcessID        string
	Plain            string
}

func printEncryptedTagsSuccess(w io.Writer, summary encryptedTagsSummary) {
	fmt.Fprintln(w, "E2E encrypted tags passed")
	fmt.Fprintf(w, "RAW message encrypted=%v plaintext_leaked=%v\n", summary.MessageEncrypted, summary.MessageLeaked)
	if summary.ProcessID != "" {
		fmt.Fprintf(w, "PROCESS pid=%s\n", summary.ProcessID)
	}
	fmt.Fprintf(w, "RESULT decrypted=true Secret=<redacted> Plain=%s\n", summary.Plain)
}

const (
	echoModule       = "pSOHJTp08z0WJ23F1iU2YQ9-nW7asB4hLNT9ME5wzmw"
	e2eMessageSecret = "message-secret-e2e"
	e2ePlain         = "plain-e2e"
)

func encryptTagsCmd() error {
	info, err := s.Client.Info()
	if err != nil {
		return fmt.Errorf("read node info: %w", err)
	}
	if info.EncryptionPublicKey == "" {
		return fmt.Errorf("node %s does not advertise encryption metadata", url)
	}
	fmt.Println("encrypt pubkey:", info.EncryptionPublicKey)

	if info.Token == "" || info.Registry == "" {
		return fmt.Errorf("core token/registry not initialized; run `go run ./examples init` first")
	}
	fmt.Println("token pid:", info.Token)
	fmt.Println("registry pid:", info.Registry)

	spawnRes, err := s.SpawnAndWait(echoModule, s.GetAddress(), nil)
	if err != nil {
		return fmt.Errorf("spawn echo: %w", err)
	}

	msgRes, err := s.SendMessageWithEncryptedParamsAndWait(
		spawnRes.Id,
		"",
		[]goarSchema.Tag{{Name: "Plain", Value: e2ePlain}},
		[]goarSchema.Tag{{Name: "Secret", Value: e2eMessageSecret}},
	)
	if err != nil {
		return fmt.Errorf("send echo message: %w", err)
	}

	output, err := verifyEchoMessage(msgRes.Message)
	if err != nil {
		return err
	}

	rawMessage, err := s.Client.GetMessage(msgRes.Id)
	if err != nil {
		return fmt.Errorf("get raw message item: %w", err)
	}
	messageEncrypted, messageLeaked := encryptedTagStored(rawMessage.Tags, vmmSchema.EncryptedTagPrefix+"Secret", e2eMessageSecret)
	if !messageEncrypted || messageLeaked {
		return fmt.Errorf("raw message encrypted=%v plaintext_leaked=%v", messageEncrypted, messageLeaked)
	}

	printEncryptedTagsSuccess(os.Stdout, encryptedTagsSummary{
		MessageEncrypted: messageEncrypted,
		MessageLeaked:    messageLeaked,
		ProcessID:        spawnRes.Id,
		Plain:            output["Plain"],
	})
	return nil
}

func verifyEchoMessage(message string) (map[string]string, error) {
	var result vmmSchema.VmmResult
	if err := json.Unmarshal([]byte(message), &result); err != nil {
		return nil, fmt.Errorf("decode message result: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("vmm result error: %s", result.Error)
	}
	output, err := outputMap(result.Output)
	if err != nil {
		return nil, err
	}
	if output["Secret"] != e2eMessageSecret || output["Plain"] != e2ePlain {
		return nil, fmt.Errorf("unexpected echo output: decrypted values did not match expected sentinels")
	}
	return output, nil
}

func encryptedTagStored(tags []goarSchema.Tag, name, plaintext string) (encrypted bool, leaked bool) {
	for _, tag := range tags {
		if tag.Value == plaintext {
			leaked = true
		}
		if tag.Name != name {
			continue
		}
		_, err := base64.StdEncoding.DecodeString(tag.Value)
		encrypted = err == nil && tag.Value != ""
		if strings.Contains(tag.Value, plaintext) {
			leaked = true
		}
	}
	return encrypted, leaked
}

func outputMap(output interface{}) (map[string]string, error) {
	raw, ok := output.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", output)
	}
	res := make(map[string]string, len(raw))
	for key, value := range raw {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected output value for %s: %T", key, value)
		}
		res[key] = str
	}
	return res, nil
}
