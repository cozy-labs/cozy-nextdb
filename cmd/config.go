package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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

func initDB(pgURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), pgURL)
}
