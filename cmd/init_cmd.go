package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a .requests/ directory with example files",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	base := ".requests"

	if _, err := os.Stat(base); err == nil {
		return fmt.Errorf("%s already exists", base)
	}

	dirs := []string{
		base,
		filepath.Join(base, "envs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(base, "envs", "local.toml"): `# Local environment variables
# Add your API base URL, tokens, and other variables here.

base_url = "http://localhost:8080"
token = "your-token-here"
`,
		filepath.Join(base, "example-http.toml"): `name = "Example HTTP request"
type = "http"
method = "GET"
url = "{{base_url}}/health"

[headers]
Accept = "application/json"
`,
		filepath.Join(base, "example-ws.toml"): `name = "Example WebSocket connection"
type = "websocket"
url = "ws://{{base_url}}/ws"

# Uncomment to send messages on connect:
# [[messages]]
# payload = '{"type": "ping"}'
# await_response = true
`,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	fmt.Println("Created .requests/ directory with example files.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .requests/envs/local.toml with your variables")
	fmt.Println("  2. Exclude env files from git:")
	fmt.Println("     echo \".requests/envs/\" >> .git/info/exclude")
	fmt.Println("  3. Run a request:")
	fmt.Println("     req run .requests/example-http.toml --env local")

	return nil
}
