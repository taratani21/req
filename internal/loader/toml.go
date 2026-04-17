package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Request struct {
	Name     string                       `toml:"name"`
	Type     string                       `toml:"type"`
	Method   string                       `toml:"method"`
	URL      string                       `toml:"url"`
	Headers  map[string]string            `toml:"headers"`
	Query    map[string]string            `toml:"query"`
	Body     Body                         `toml:"body"`
	Messages []Message                    `toml:"messages"`
	Variants map[string]map[string]string `toml:"variants"`
}

type Body struct {
	Data string `toml:"data"`
}

type Message struct {
	Payload       string `toml:"payload"`
	AwaitResponse bool   `toml:"await_response"`
}

type ChainFile struct {
	Name  string      `toml:"name"`
	Steps []ChainStep `toml:"steps"`
}

type ChainStep struct {
	Request string            `toml:"request"`
	Extract map[string]string `toml:"extract"`
}

func LoadRequest(path string) (*Request, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading request file: %w", err)
	}

	var req Request
	if err := toml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parsing request file %s: %w", path, err)
	}

	return &req, nil
}

func LoadEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing env file %s: %w", path, err)
	}

	env := make(map[string]string, len(raw))
	for k, v := range raw {
		env[k] = fmt.Sprintf("%v", v)
	}

	return env, nil
}

func LoadChain(path string) (*ChainFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading chain file: %w", err)
	}

	var chain ChainFile
	if err := toml.Unmarshal(data, &chain); err != nil {
		return nil, fmt.Errorf("parsing chain file %s: %w", path, err)
	}

	return &chain, nil
}

func LoadEnvHierarchical(startDir, name string) (map[string]string, error) {
	filename := name + ".toml"
	var found []string
	dir := startDir
	for {
		path := filepath.Join(dir, "envs", filename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no envs/%s.toml found in %s or any ancestor", name, startDir)
	}
	merged := make(map[string]string)
	for i := len(found) - 1; i >= 0; i-- {
		e, err := LoadEnv(found[i])
		if err != nil {
			return nil, err
		}
		for k, v := range e {
			merged[k] = v
		}
	}
	return merged, nil
}
