package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func initConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("COZY_NEXTDB")
	viper.AutomaticEnv()

	viper.SetConfigName("nextdb")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/cozy/")
	viper.AddConfigPath("$HOME/.cozy")
	viper.AddConfigPath(".")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	err := viper.ReadInConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		err = nil // No config file is OK
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read configuration file: %s\n", err)
		os.Exit(1)
	}
}
