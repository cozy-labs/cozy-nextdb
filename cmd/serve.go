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
		host := viper.GetString("host")
		port := viper.GetInt("port")
		certFile := viper.GetString("tls.cert")
		keyFile := viper.GetString("tls.key")

		return web.ListenAndServe(host, port, certFile, keyFile)
	},
}
