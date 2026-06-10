package usage

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// pricingJSON is LiteLLM's pricing database, distilled to the providers we read
// and vendored at build time so cost stays accurate without any runtime network
// call (regenerate with scripts/gen-pricing.py). Values are USD per million
// tokens, keyed by a normalized model name.
//
//go:embed pricing_data.json
var pricingJSON []byte

var (
	pricingOnce  sync.Once
	pricingTable map[string]Pricing
)

func litellmTable() map[string]Pricing {
	pricingOnce.Do(func() {
		pricingTable = map[string]Pricing{}
		_ = json.Unmarshal(pricingJSON, &pricingTable)
	})
	return pricingTable
}

// VendoredPricingCount returns the number of model entries bundled into the
// binary from LiteLLM's pricing data. It is used by diagnostics and tests.
func VendoredPricingCount() int {
	return len(litellmTable())
}

// litellmPricing resolves a model against the vendored LiteLLM table: an exact
// (case-insensitive) match first, then the longest table key that is a
// substring of the model name (so dated or suffixed ids like
// "claude-sonnet-4-5-20250930" still match "claude-sonnet-4-5"). ok is false
// when nothing matches, so callers can fall back to the built-in defaults.
func litellmPricing(model string) (Pricing, bool) {
	t := litellmTable()
	m := strings.ToLower(model)
	if p, ok := t[m]; ok {
		return p, true
	}
	var best string
	for k := range t {
		if len(k) > len(best) && strings.Contains(m, k) {
			best = k
		}
	}
	if best != "" {
		return t[best], true
	}
	return Pricing{}, false
}
