package interpolate

import (
	"fmt"
	"regexp"
	"strings"
)

var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

func FindVariables(input string) []string {
	matches := varPattern.FindAllStringSubmatch(input, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

func Interpolate(input string, vars map[string]string) (string, error) {
	var unresolved []string

	result := varPattern.ReplaceAllStringFunc(input, func(match string) string {
		name := match[2 : len(match)-2]
		if val, ok := vars[name]; ok {
			return val
		}
		unresolved = append(unresolved, name)
		return match
	})

	if len(unresolved) > 0 {
		return "", fmt.Errorf("unresolved variable %q\nhint: set it with --var %s=<value> or define it in your env file",
			unresolved[0], unresolved[0])
	}

	return result, nil
}

func ResolveVars(cliVars, extracted, envVars map[string]string) map[string]string {
	resolved := make(map[string]string)

	for k, v := range envVars {
		resolved[k] = v
	}
	for k, v := range extracted {
		resolved[k] = v
	}
	for k, v := range cliVars {
		resolved[k] = v
	}

	return resolved
}

func InterpolateRequest(url string, headers, query map[string]string, bodyData string, vars map[string]string) (
	newURL string, newHeaders, newQuery map[string]string, newBody string, err error,
) {
	newURL, err = Interpolate(url, vars)
	if err != nil {
		return "", nil, nil, "", fmt.Errorf("in url: %w", err)
	}

	newHeaders = make(map[string]string, len(headers))
	for k, v := range headers {
		newHeaders[k], err = Interpolate(v, vars)
		if err != nil {
			return "", nil, nil, "", fmt.Errorf("in header %q: %w", k, err)
		}
	}

	newQuery = make(map[string]string, len(query))
	for k, v := range query {
		newQuery[k], err = Interpolate(v, vars)
		if err != nil {
			return "", nil, nil, "", fmt.Errorf("in query param %q: %w", k, err)
		}
	}

	if bodyData != "" {
		newBody, err = Interpolate(bodyData, vars)
		if err != nil {
			return "", nil, nil, "", fmt.Errorf("in body: %w", err)
		}
	}

	return newURL, newHeaders, newQuery, newBody, nil
}

func ParseCLIVars(rawVars []string) (map[string]string, error) {
	result := make(map[string]string, len(rawVars))
	for _, kv := range rawVars {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --var format %q, expected key=value", kv)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
