package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

var (
	envName     string
	vars        []string
	verbose     bool
	timeout     time.Duration
	variantName string
)

var rootCmd = &cobra.Command{
	Use:   "req",
	Short: "Terminal-native HTTP & WebSocket client",
	Long:  "Run HTTP and WebSocket requests defined in TOML files from the terminal.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "Load environment from envs/<name>.toml, merging ancestors up to .requests/")
	rootCmd.PersistentFlags().StringArrayVar(&vars, "var", nil, "Override or inject a variable (key=value, repeatable)")
	rootCmd.PersistentFlags().StringVar(&variantName, "variant", "", "Select a named variant from the request file")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Print request details to stderr")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout (e.g. 30s, 5m)")
}
