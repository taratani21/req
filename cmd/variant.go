package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/taratani21/req/internal/loader"
)

// resolveVariant returns the variant vars for the given name from req.Variants.
// Returns (nil, nil) when variantName is empty.
// When variantName is set but not found: returns an error with the list of available
// variants, unless silent is true (then returns nil, nil — used by chain so missing
// variants in a single step don't abort the whole run).
func resolveVariant(req *loader.Request, variantName string, silent bool) (map[string]string, error) {
	if variantName == "" {
		return nil, nil
	}
	if v, ok := req.Variants[variantName]; ok {
		return v, nil
	}
	if silent {
		return nil, nil
	}
	available := make([]string, 0, len(req.Variants))
	for name := range req.Variants {
		available = append(available, name)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return nil, fmt.Errorf("unknown variant %q (this request defines no variants)", variantName)
	}
	return nil, fmt.Errorf("unknown variant %q (available: %s)", variantName, strings.Join(available, ", "))
}
