package cmd

import (
	"github.com/cozy-labs/cozy-nextdb/web"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the dedup and listens for HTTP calls",
	Long: `Starts the dedup and listens for HTTP calls
It will accept HTTP requests on localhost:9000 by default.
Use the --port and --host flags to change the listening option.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := &web.Server{
			Host:     viper.GetString("host"),
			Port:     viper.GetInt("port"),
			CertFile: viper.GetString("tls.cert"),
			KeyFile:  viper.GetString("tls.key"),
		}

		pg, err := initPG(viper.GetString("pg.url"))
		if err != nil {
			return err
		}
		server.PG = pg
		defer pg.Close()

		logger, err := initLogger()
		if err != nil {
			return err
		}
		server.Logger = logger

		return server.ListenAndServe()
	},
}
