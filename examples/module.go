package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	echoSchema "github.com/cryptowizard0/encrypttags/echo/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
)

func genModule() (string, error) {
	itemID, err := writeLocalModule(echoSchema.ModuleFormat)
	if err != nil {
		return "", fmt.Errorf("generate and save module failed: %w", err)
	}
	fmt.Println("generate and save module success, id", itemID)
	return itemID, nil
}

func writeLocalModule(moduleFormat string) (string, error) {
	tags, err := utils.ModuleToTags(hymxSchema.Module{
		Base:         hymxSchema.DefaultBaseModule,
		ModuleFormat: moduleFormat,
	})
	if err != nil {
		return "", fmt.Errorf("module tags %s: %w", moduleFormat, err)
	}
	item, err := s.Bundler.CreateAndSignItem([]byte{}, "", "", tags)
	if err != nil {
		return "", fmt.Errorf("sign module %s: %w", moduleFormat, err)
	}
	if err := os.MkdirAll("mod", 0o755); err != nil {
		return "", err
	}
	by, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	path := filepath.Join("mod", fmt.Sprintf("mod-%s.json", item.Id))
	if err := os.WriteFile(path, by, 0o644); err != nil {
		return "", err
	}
	return item.Id, nil
}
