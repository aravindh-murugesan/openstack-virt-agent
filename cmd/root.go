package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/version"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var Config config.Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "openstack-virt-agent",
	Short:   "Virt Agent for OpenStack",
	Long:    `Virt Agent enforces and adapts features that are not yet available on openstack
or the behavior in openstack may be too rigid for cloud operators to function.`,
	Version: version.Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/openstack-virt-agent/config.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file %s does not exist\n", cfgFile)
			os.Exit(1)
		}
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory or /etc
		viper.AddConfigPath("/etc/openstack-virt-agent/")
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&Config); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to decode into struct, %v\n", err)
		os.Exit(1)
	}

	if err := Config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}
}

func setupLogger() {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(Config.LogLevel)); err != nil {
		lvl = slog.LevelInfo // fallback if unparseable or empty
	}

	var handler slog.Handler
	if Config.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	} else {
		// tint is used for development/pretty-printing
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      lvl,
			TimeFormat: time.RFC3339,
		})
	}
	slog.SetDefault(slog.New(handler))
}
