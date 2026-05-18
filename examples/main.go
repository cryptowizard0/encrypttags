package main

import (
	"fmt"
	"os"

	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/sdk"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
)

const defaultPrivateKey = "0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b"

var (
	url = envOrDefault("ENCRYPTTAGS_URL", "http://127.0.0.1:8080")
	s   *sdk.SDK

	mainNode = registrySchema.Node{
		Name: "encrypttags-e2e",
		Desc: "encrypted tags local e2e node",
		URL:  url,
	}
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("please input cmd, ex: init, module, spawn-echo, encrypttags, stop-resume, checkpoint, checkpoint-restore")
		os.Exit(1)
	}

	initSDK()

	switch os.Args[1] {
	case "init":
		tokenPid, err := initToken()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		initRegistry(tokenPid, mainNode)
	case "module":
		if _, err := genModule(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "encrypttags":
		if err := encryptTagsCmd(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "spawn-echo":
		if err := spawnEchoCmd(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "stop-resume":
		if err := stopResumeCmd(os.Stdout, os.Args[2:]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "checkpoint":
		if err := checkpointCmd(os.Stdout, os.Args[2:]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "checkpoint-restore":
		if err := checkpointRestoreCmd(os.Stdout, os.Args[2:]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown cmd: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func initSDK() {
	if s != nil {
		return
	}

	prvKey := envOrDefault("ENCRYPTTAGS_PRIVATE_KEY", defaultPrivateKey)
	signer, err := goether.NewSigner(prvKey)
	if err != nil {
		panic(err)
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		panic(err)
	}
	s = sdk.NewFromBundler(url, bundler)
}

func envOrDefault(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}
