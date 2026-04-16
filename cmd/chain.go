package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taratani21/req/internal/extract"
	"github.com/taratani21/req/internal/interpolate"
	"github.com/taratani21/req/internal/loader"
	"github.com/taratani21/req/internal/runner"
)

var chainCmd = &cobra.Command{
	Use:   "chain <file>",
	Short: "Run a chain of requests with value extraction between steps",
	Args:  cobra.ExactArgs(1),
	RunE:  runChain,
}

func init() {
	rootCmd.AddCommand(chainCmd)
}

func runChain(cmd *cobra.Command, args []string) error {
	chainFile := args[0]
	chainDir := filepath.Dir(chainFile)

	chain, err := loader.LoadChain(chainFile)
	if err != nil {
		return err
	}

	// Load env file if specified
	var envVars map[string]string
	if envName != "" {
		envPath := filepath.Join(chainDir, "envs", envName+".toml")
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

	// Validate all referenced request files exist before running
	for i, step := range chain.Steps {
		reqPath := filepath.Join(chainDir, step.Request)
		if _, err := os.Stat(reqPath); err != nil {
			return fmt.Errorf("step %d: request file %q not found: %w", i+1, reqPath, err)
		}
	}

	extracted := make(map[string]string)

	for i, step := range chain.Steps {
		reqPath := filepath.Join(chainDir, step.Request)

		req, err := loader.LoadRequest(reqPath)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		if req.Type != "http" {
			return fmt.Errorf("step %d: chain only supports http requests, got %q", i+1, req.Type)
		}

		// Resolve variables for this step
		resolved := interpolate.ResolveVars(cliVars, nil, extracted, envVars)

		// Interpolate all fields
		url, headers, query, body, err := interpolate.InterpolateRequest(
			req.URL, req.Headers, req.Query, req.Body.Data, resolved,
		)
		if err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "--- Step %d: %s ---\n", i+1, req.Name)
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
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("step %d: reading response: %w", i+1, err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "HTTP %s\n", resp.Status)
			if i < len(chain.Steps)-1 {
				fmt.Fprintf(os.Stderr, "%s\n\n", string(respBody))
			}
		}

		// Check status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			os.Stdout.Write(respBody)
			os.Exit(1)
		}

		// Extract values for next steps
		if len(step.Extract) > 0 {
			newVars, err := extract.All(respBody, step.Extract)
			if err != nil {
				return fmt.Errorf("step %d: %w", i+1, err)
			}
			for k, v := range newVars {
				extracted[k] = v
			}
		}

		// Write last step's response to stdout
		if i == len(chain.Steps)-1 {
			os.Stdout.Write(respBody)
		}
	}

	return nil
}
