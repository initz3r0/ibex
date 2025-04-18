package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/spf13/viper"
	"golang.org/x/exp/slog"
)

var configLock sync.Mutex

func Init() {
	configLock.Lock()
	defer configLock.Unlock()

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		slog.Error("Unable to locate users home directory")
		return
	}

	configPath := "/.config/ibex/"
	defaultOutputPathDir := filepath.Join(homeDirectory, "/ibex/")

	if runtime.GOOS == "windows" {
		configPath = "\\.ibex\\"
	}

	viperConfigDirectory := filepath.Join(homeDirectory, configPath)

	// Make our default ibex output config directory
	if _, err := os.Stat(viperConfigDirectory); os.IsNotExist(err) {
		err := os.MkdirAll(viperConfigDirectory, os.ModePerm)
		if err != nil {
			slog.Error("Failed to create config directory.", "Error", err)
			return
		}
	}

	// Make our default decryption output directory
	if _, err := os.Stat(defaultOutputPathDir); os.IsNotExist(err) {
		err := os.MkdirAll(defaultOutputPathDir, os.ModePerm)
		if err != nil {
			slog.Info("Failed to create decryption output directory.", "Error", err)
			return
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(viperConfigDirectory)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.WriteConfigAs(filepath.Join(viperConfigDirectory, "config.toml"))
		} else {
			slog.Info("Reading in the ibex viper config.")
		}
	}

	viper.SetDefault("first_run", "true")
	viper.SetDefault("log_level", "Info")
	viper.SetDefault("output_path", defaultOutputPathDir)

	if err := viper.WriteConfig(); err != nil {
		slog.Error("Failed to write ibex viper config.", "Error", err)
		return
	}

}
