package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taratani21/req/internal/interpolate"
	"github.com/taratani21/req/internal/loader"
	"github.com/taratani21/req/internal/runner"
)

var runCmd = &cobra.Command{
	Use:   "run <file>",
	Short: "Run an HTTP request from a TOML file",
	Args:  cobra.ExactArgs(1),
	RunE:  runRequest,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRequest(cmd *cobra.Command, args []string) error {
	reqFile := args[0]

	req, err := loader.LoadRequest(reqFile)
	if err != nil {
		return err
	}

	if req.Type != "http" {
		return fmt.Errorf("expected type \"http\", got %q (use 'req ws' for websocket requests)", req.Type)
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

	// Resolve variables (cli > env, no extracted vars for single run)
	resolved := interpolate.ResolveVars(cliVars, nil, nil, envVars)

	// Interpolate all fields
	url, headers, query, body, err := interpolate.InterpolateRequest(
		req.URL, req.Headers, req.Query, req.Body.Data, resolved,
	)
	if err != nil {
		return err
	}

	// Print verbose info to stderr
	if verbose {
		fmt.Fprintf(os.Stderr, "%s %s\n", req.Method, url)
		for k, v := range headers {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Execute the request
	resp, err := runner.Run(&runner.Request{
		Method:  req.Method,
		URL:     url,
		Headers: headers,
		Query:   query,
		Body:    body,
		Timeout: timeout,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Print verbose response info to stderr
	if verbose {
		fmt.Fprintf(os.Stderr, "HTTP %s\n", resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v[0])
		}
		fmt.Fprintln(os.Stderr)
	}

	// Write body to stdout
	io.Copy(os.Stdout, resp.Body)

	// Exit code 1 for non-2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		os.Exit(1)
	}

	return nil
}
