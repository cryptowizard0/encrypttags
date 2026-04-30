package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type encryptedTagsSummary struct {
	SpawnEncrypted   bool
	SpawnLeaked      bool
	MessageEncrypted bool
	MessageLeaked    bool
	ReservedRejected bool
	Plain            string
}

func printEncryptedTagsSuccess(w io.Writer, summary encryptedTagsSummary) {
	fmt.Fprintln(w, "E2E encrypted tags passed")
	fmt.Fprintf(w, "RAW spawn encrypted=%v plaintext_leaked=%v\n", summary.SpawnEncrypted, summary.SpawnLeaked)
	fmt.Fprintf(w, "RAW message encrypted=%v plaintext_leaked=%v\n", summary.MessageEncrypted, summary.MessageLeaked)
	fmt.Fprintf(w, "RESULT decrypted=true Secret=<redacted> SpawnSecret=<redacted> Plain=%s\n", summary.Plain)
	fmt.Fprintf(w, "reserved encrypted tag rejected=%v\n", summary.ReservedRejected)
}

const (
	echoModule       = "pSOHJTp08z0WJ23F1iU2YQ9-nW7asB4hLNT9ME5wzmw"
	e2eSpawnSecret   = "spawn-secret-e2e"
	e2eMessageSecret = "message-secret-e2e"
	e2ePlain         = "plain-e2e"
)

func encryptTagsCmd() error {
	info, err := s.Client.Info()
	if err != nil {
		return fmt.Errorf("read node info: %w", err)
	}
	if info.EncryptionPublicKey == "" || info.EncryptionKeyType == "" {
		return fmt.Errorf("node %s does not advertise encryption metadata", url)
	}
	fmt.Println("encrypt keytype:", info.EncryptionKeyType)
	fmt.Println("encrypt pubkey:", info.EncryptionPublicKey)

	if info.Token == "" || info.Registry == "" {
		return fmt.Errorf("core token/registry not initialized; run `go run ./examples init` first")
	}
	fmt.Println("token pid:", info.Token)
	fmt.Println("registry pid:", info.Registry)

	spawnRes, err := s.SpawnAndWait(echoModule, s.GetAddress(), []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "SpawnSecret", Value: e2eSpawnSecret},
	})
	if err != nil {
		return fmt.Errorf("spawn echo: %w", err)
	}

	rawSpawn, err := s.Client.GetMessage(spawnRes.Id)
	if err != nil {
		return fmt.Errorf("get raw spawn item: %w", err)
	}
	spawnEncrypted, spawnLeaked := encryptedTagStored(rawSpawn.Tags, tagcrypto.EncryptedTagPrefix+"SpawnSecret", e2eSpawnSecret, info.EncryptionKeyType)
	if !spawnEncrypted || spawnLeaked {
		return fmt.Errorf("raw spawn encrypted=%v plaintext_leaked=%v", spawnEncrypted, spawnLeaked)
	}

	msgRes, err := s.SendMessageAndWait(spawnRes.Id, "", []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: e2eMessageSecret},
		{Name: "Plain", Value: e2ePlain},
	})
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
	messageEncrypted, messageLeaked := encryptedTagStored(rawMessage.Tags, tagcrypto.EncryptedTagPrefix+"Secret", e2eMessageSecret, info.EncryptionKeyType)
	if !messageEncrypted || messageLeaked {
		return fmt.Errorf("raw message encrypted=%v plaintext_leaked=%v", messageEncrypted, messageLeaked)
	}

	reservedRejected := false
	if err := sendReservedEncryptedTagToNode(spawnRes.Id); err != nil && strings.Contains(err.Error(), "400") {
		reservedRejected = true
	}
	if !reservedRejected {
		return fmt.Errorf("reserved encrypted tag rejected=false")
	}

	printEncryptedTagsSuccess(os.Stdout, encryptedTagsSummary{
		SpawnEncrypted:   spawnEncrypted,
		SpawnLeaked:      spawnLeaked,
		MessageEncrypted: messageEncrypted,
		MessageLeaked:    messageLeaked,
		ReservedRejected: reservedRejected,
		Plain:            output["Plain"],
	})
	return nil
}

func sendReservedEncryptedTagToNode(pid string) error {
	msgTags, err := utils.MessageToTags(hymxSchema.Message{
		Base: hymxSchema.DefaultBaseMessage,
	})
	if err != nil {
		return err
	}
	msgTags = utils.MergeTags(msgTags, []goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Type", Value: tagcrypto.CipherValuePrefix + ":" + tagcrypto.KeyTypeEthereumECIES + ":bad"},
	})
	item, err := s.Bundler.CreateAndSignItem([]byte{}, pid, "", msgTags)
	if err != nil {
		return err
	}
	_, _, err = s.Client.Send(item.Binary)
	return err
}

func verifyEchoMessage(message string) (map[string]string, error) {
	var result vmmSchema.VmmResult
	if err := json.Unmarshal([]byte(message), &result); err != nil {
		return nil, fmt.Errorf("decode message result: %w", err)
	}
	output, err := outputMap(result.Output)
	if err != nil {
		return nil, err
	}
	if output["SpawnSecret"] != e2eSpawnSecret || output["Secret"] != e2eMessageSecret || output["Plain"] != e2ePlain {
		return nil, fmt.Errorf("unexpected echo output: decrypted values did not match expected sentinels")
	}
	return output, nil
}

func encryptedTagStored(tags []goarSchema.Tag, name, plaintext, keyType string) (encrypted bool, leaked bool) {
	for _, tag := range tags {
		if tag.Value == plaintext {
			leaked = true
		}
		if tag.Name != name {
			continue
		}
		expectedPrefix := tagcrypto.CipherValuePrefix + ":" + keyType + ":"
		encrypted = strings.HasPrefix(tag.Value, expectedPrefix)
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
