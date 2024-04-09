package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "cozy-nextdb <command>",
	Short: "cozy-nextdb is the main command",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Display the usage/help by default
		return cmd.Usage()
	},
	// Do not display usage on error
	SilenceUsage: true,
	// We have our own way to display error messages
	SilenceErrors: true,
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)
	rootFlags := RootCmd.PersistentFlags()
	rootFlags.StringVarP(&cfgFile, "config", "c", "", "path to the configuration file")
	rootFlags.String("host", "localhost", "server host")
	checkNoErr(viper.BindPFlag("host", rootFlags.Lookup("host")))
	rootFlags.IntP("port", "p", 7654, "server port")
	checkNoErr(viper.BindPFlag("port", rootFlags.Lookup("port")))
	rootFlags.StringP("log-level", "L", "info", "set the logger level")
	checkNoErr(viper.BindPFlag("log.level", rootFlags.Lookup("log-level")))
	rootFlags.Bool("log-syslog", false, "use the local syslog for logging")
	checkNoErr(viper.BindPFlag("log.syslog", rootFlags.Lookup("log-syslog")))

	docCmdGroup.AddCommand(manDocCmd)
	docCmdGroup.AddCommand(markdownDocCmd)
	RootCmd.AddCommand(docCmdGroup)

	serveFlags := serveCmd.Flags()
	serveFlags.String("cert-file", "", "the certificate file for TLS")
	checkNoErr(viper.BindPFlag("tls.cert", serveFlags.Lookup("cert-file")))
	serveFlags.String("key-file", "", "the key file for TLS")
	checkNoErr(viper.BindPFlag("tls.key", serveFlags.Lookup("key-file")))
	RootCmd.AddCommand(serveCmd)

	usageFunc := RootCmd.UsageFunc()
	RootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_ = usageFunc(cmd)
		return nil
	})
}

func checkNoErr(err error) {
	if err != nil {
		panic(err)
	}
}
