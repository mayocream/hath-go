package main

import (
	_ "embed"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mayocream/hath-go/server"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

//go:embed example.config.yaml
var exampleCfg []byte

func parseCfg(file string) (*server.Config, error) {
	if file == "" || file == "home" {
		fmt.Println("Not specify exact config file, fallback using ~/.hath/config.yaml")
		hd, err := homedir.Dir()
		if err != nil {
			return nil, err
		}

		file = filepath.Join(hd, ".hath", "config.yaml")
	}

	baseDir := filepath.Dir(file)

	if _, err := os.Stat(baseDir); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(baseDir, 0755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(file, exampleCfg, 0755); err != nil {
			return nil, err
		}
	}

	viper.SetEnvPrefix("hath")
	viper.EnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	viper.SetConfigType("yaml")
	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	conf := new(server.Config)
	if err := viper.Unmarshal(conf); err != nil {
		return nil, err
	}

	fmt.Printf("Start with ClientID: %s, DB: %s \n", conf.ClientID, conf.DBFile)
	if conf.ClientID == "" || conf.ClientKey == "" {
		return nil, errors.New("missing id/key")
	}

	if conf.DBFile == "" {
		conf.DBFile = filepath.Join(baseDir, ".hath", "hv.ldb")
		fmt.Println("Using default db data path: ", conf.DBFile)
	}

	return conf, nil
}
