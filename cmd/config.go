package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"log/syslog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
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

func initLogger() (*slog.Logger, error) {
	logLevel := strings.ToLower(viper.GetString("log.level"))
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("cannot parse log level: %s", logLevel)
	}

	var handler slog.Handler
	if viper.GetBool("log.syslog") {
		w, err := syslog.Dial("", "", syslog.LOG_INFO, "cozy-nextdb")
		if err != nil {
			return nil, fmt.Errorf("cannot initialize syslog: %w", err)
		}
		opts := &slog.HandlerOptions{Level: level}
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      level,
			TimeFormat: time.DateTime,
		})
	}
	return slog.New(handler), nil
}
