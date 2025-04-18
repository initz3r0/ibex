/*
Copyright © 2023 <initz3ro>
*/
package main

import (
	"ibex/cmd"
	"ibex/internal/config"
	"log/slog"

	"github.com/spf13/viper"
)

var version string

func main() {

	config.Init()
	firstRun := viper.GetString("first_run")
	if firstRun == "true" {

		configPath := viper.ConfigFileUsed()
		slog.Info("Creating ibex configuration file", "ConfigurationPath", configPath)

		viper.Set("version", version)
		viper.Set("first_run", "false")

		if err := viper.WriteConfig(); err != nil {
			slog.Error("Failed to update ibex viper configuration", "Error", err)
		}

		slog.Info("Configuration completed.")

	}

	if version != viper.GetString("version") {
		viper.Set("version", version)
	}

	cmd.Execute()
}
