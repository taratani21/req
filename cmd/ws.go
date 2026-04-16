package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taratani21/req/internal/interpolate"
	"github.com/taratani21/req/internal/loader"
	"github.com/taratani21/req/internal/ws"
)

var noInteractive bool

var wsCmd = &cobra.Command{
	Use:   "ws <file>",
	Short: "Connect a WebSocket request and enter interactive mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runWebSocket,
}

func init() {
	wsCmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "Send defined messages only, then disconnect")
	rootCmd.AddCommand(wsCmd)
}

func runWebSocket(cmd *cobra.Command, args []string) error {
	reqFile := args[0]

	req, err := loader.LoadRequest(reqFile)
	if err != nil {
		return err
	}

	if req.Type != "websocket" {
		return fmt.Errorf("expected type \"websocket\", got %q (use 'req run' for http requests)", req.Type)
	}

	// Load env file if specified
	var envVars map[string]string
	if envName != "" {
		envPath := filepath.Join(filepath.Dir(reqFile), "envs", envName+".toml")
		envVars, err = loader.LoadEnv(envPath)
		if err != nil {
			return fmt.Errorf("loading env %q: %w", envName, err)
		}
	}

	// Parse CLI vars
	cliVars, err := interpolate.ParseCLIVars(vars)
	if err != nil {
		return err
	}

	// Resolve variables
	resolved := interpolate.ResolveVars(cliVars, nil, envVars)

	// Interpolate URL and headers
	url, err := interpolate.Interpolate(req.URL, resolved)
	if err != nil {
		return fmt.Errorf("in url: %w", err)
	}

	headers := make(map[string]string, len(req.Headers))
	for k, v := range req.Headers {
		headers[k], err = interpolate.Interpolate(v, resolved)
		if err != nil {
			return fmt.Errorf("in header %q: %w", k, err)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "WS %s\n", url)
		for k, v := range headers {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Connect
	conn, err := ws.Connect(url, headers, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	if verbose {
		fmt.Fprintln(os.Stderr, "Connected.")
	}

	// Send predefined messages
	if len(req.Messages) > 0 {
		messages := make([]ws.Message, len(req.Messages))
		for i, m := range req.Messages {
			payload, err := interpolate.Interpolate(m.Payload, resolved)
			if err != nil {
				return fmt.Errorf("in message %d payload: %w", i+1, err)
			}
			messages[i] = ws.Message{
				Payload:       payload,
				AwaitResponse: m.AwaitResponse,
			}
		}

		if err := ws.SendMessages(conn, messages, timeout); err != nil {
			return err
		}
	}

	// If no-interactive, we're done
	if noInteractive {
		return nil
	}

	// Enter interactive mode
	return ws.Interactive(conn, os.Stdin, os.Stdout)
}
