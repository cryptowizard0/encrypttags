package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	echoSchema "github.com/cryptowizard0/encrypttags/echo/schema"

	"github.com/cryptowizard0/encrypttags/echo"
	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/server"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

var log = common.NewLog("encrypttags-cmd")

func main() {
	cli.VersionFlag = flagVersion

	app := &cli.App{
		Name:    schema.DataProtocol,
		Version: nodeSchema.NodeVersion,
		Flags:   flags,
		Action:  action,
	}

	if err := app.Run(os.Args); err != nil {
		log.Error("run server failed", "err", err)
	}
}

func action(c *cli.Context) error {
	configPath, err := resolveConfigPath(c.String("config"))
	if err != nil {
		return err
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return run(c)
}

func resolveConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	for _, candidate := range []string{"./config.yaml", "./cmd/config.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("config file not found; pass --config or run from repo root/cmd directory")
}

func run(c *cli.Context) (err error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	port, ginMode, redisURL, arweaveURL, hymxURL, bundler, nodeInfo, decryptor, err := LoadNodeConfig()
	if err != nil {
		return err
	}

	gin.SetMode(ginMode)
	if ginMode == "release" {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler))
	}

	n := node.New(decryptor, bundler, redisURL, arweaveURL, hymxURL, nodeInfo, nil)
	s := server.New(n, nil)
	if err = s.Mount(echoSchema.ModuleFormat, echo.Spawn); err != nil {
		return err
	}

	s.Run(port, c.String("mode"))
	log.Info("server is running", "protocol version", schema.Variant, "node version", nodeSchema.NodeVersion, "wallet", bundler.Address, "port", port)

	<-signals
	s.Close()

	return nil
}
