package usage

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// pricingJSON is LiteLLM's pricing database, distilled to the providers we read
// and vendored at build time so cost stays accurate without any runtime network
// call (regenerate with scripts/gen-pricing.py). Values are USD per million
// tokens, keyed by a normalized model name.
//
//go:embed pricing_data.json
var pricingJSON []byte

//go:embed pricing_metadata.json
var pricingMetadataJSON []byte

var (
	pricingOnce  sync.Once
	pricingTable map[string]Pricing
)

const LiteLLMPricingSourceURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

type PricingSnapshotMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	Source        string `json:"source"`
	SourceURL     string `json:"source_url"`
	GeneratedAt   string `json:"generated_at"`
	ModelCount    int    `json:"model_count"`
	DataSHA256    string `json:"data_sha256"`
}

type PricingUpdateCheck struct {
	Current       PricingSnapshotMetadata `json:"current"`
	SourceURL     string                  `json:"source_url"`
	LatestModels  int                     `json:"latest_models"`
	LatestSHA256  string                  `json:"latest_sha256"`
	UpToDate      bool                    `json:"up_to_date"`
	AddedModels   int                     `json:"added_models"`
	RemovedModels int                     `json:"removed_models"`
	ChangedModels int                     `json:"changed_models"`
}

func litellmTable() map[string]Pricing {
	pricingOnce.Do(func() {
		pricingTable = map[string]Pricing{}
		_ = json.Unmarshal(pricingJSON, &pricingTable)
	})
	return pricingTable
}

func PricingMetadata() PricingSnapshotMetadata {
	var meta PricingSnapshotMetadata
	_ = json.Unmarshal(pricingMetadataJSON, &meta)
	if meta.ModelCount == 0 {
		meta.ModelCount = VendoredPricingCount()
	}
	if meta.DataSHA256 == "" {
		meta.DataSHA256 = pricingTableSHA256(litellmTable())
	}
	return meta
}

// VendoredPricingCount returns the number of model entries bundled into the
// binary from LiteLLM's pricing data. It is used by diagnostics and tests.
func VendoredPricingCount() int {
	return len(litellmTable())
}

func CheckPricingUpdate(ctx context.Context) (PricingUpdateCheck, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LiteLLMPricingSourceURL, nil)
	if err != nil {
		return PricingUpdateCheck{}, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return PricingUpdateCheck{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PricingUpdateCheck{}, fmt.Errorf("fetch pricing source: %s", resp.Status)
	}
	var raw map[string]map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return PricingUpdateCheck{}, fmt.Errorf("decode pricing source: %w", err)
	}
	latest := distillLiteLLMPricing(raw)
	current := litellmTable()
	added, removed, changed := comparePricingTables(current, latest)
	latestHash := pricingTableSHA256(latest)
	meta := PricingMetadata()
	return PricingUpdateCheck{
		Current:       meta,
		SourceURL:     LiteLLMPricingSourceURL,
		LatestModels:  len(latest),
		LatestSHA256:  latestHash,
		UpToDate:      latestHash == meta.DataSHA256,
		AddedModels:   added,
		RemovedModels: removed,
		ChangedModels: changed,
	}, nil
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

var (
	litellmPrefix = regexp.MustCompile(`^(us|eu|apac|au|ca|sa|global|anthropic|bedrock|vertex_ai|vertex|azure_ai|azure|openai|gemini|google|fireworks_ai|openrouter|converse|invoke)[./]`)
	litellmSuffix = regexp.MustCompile(`(-v\d+:\d+|:\d+|-v\d+|@\d{8})$`)
	litellmFamily = regexp.MustCompile(`(claude|gpt-|gpt4|o1|o3|o4|codex|gemini)`)
)

func distillLiteLLMPricing(raw map[string]map[string]any) map[string]Pricing {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sortByPlainness(keys)
	out := map[string]Pricing{}
	for _, key := range keys {
		v := raw[key]
		mode, _ := v["mode"].(string)
		if mode != "" && mode != "chat" && mode != "completion" {
			continue
		}
		inp, okIn := numberField(v, "input_cost_per_token")
		outp, okOut := numberField(v, "output_cost_per_token")
		if !okIn || !okOut {
			continue
		}
		provider, _ := v["litellm_provider"].(string)
		if !isLiteLLMFamily(key, provider) {
			continue
		}
		name := normalizeLiteLLMKey(key)
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		entry := Pricing{
			InputPerMillion:  round6(inp * 1_000_000),
			OutputPerMillion: round6(outp * 1_000_000),
		}
		if cacheRead, ok := numberField(v, "cache_read_input_token_cost"); ok {
			entry.CacheReadPerMillion = round6(cacheRead * 1_000_000)
		}
		if cacheCreate, ok := numberField(v, "cache_creation_input_token_cost"); ok {
			entry.CacheWritePerMillion = round6(cacheCreate * 1_000_000)
		}
		out[name] = entry
	}
	return out
}

func sortByPlainness(keys []string) {
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}
		return keys[i] < keys[j]
	})
}

func normalizeLiteLLMKey(key string) string {
	k := strings.ToLower(key)
	if strings.Contains(k, "/") {
		parts := strings.Split(k, "/")
		k = parts[len(parts)-1]
	}
	for {
		next := litellmPrefix.ReplaceAllString(k, "")
		if next == k {
			break
		}
		k = next
	}
	for {
		next := litellmSuffix.ReplaceAllString(k, "")
		if next == k {
			break
		}
		k = next
	}
	return k
}

func isLiteLLMFamily(key, provider string) bool {
	switch provider {
	case "anthropic", "openai", "gemini", "vertex_ai-language-models":
		return true
	default:
		return litellmFamily.MatchString(strings.ToLower(key))
	}
}

func numberField(v map[string]any, key string) (float64, bool) {
	n, ok := v[key].(float64)
	return n, ok
}

func round6(v float64) float64 {
	return float64(int64(v*1_000_000+0.5)) / 1_000_000
}

func pricingTableSHA256(table map[string]Pricing) string {
	canonical := make(map[string]map[string]float64, len(table))
	for model, price := range table {
		entry := map[string]float64{
			"input_per_million":  price.InputPerMillion,
			"output_per_million": price.OutputPerMillion,
		}
		if price.CacheReadPerMillion != 0 {
			entry["cache_read_per_million"] = price.CacheReadPerMillion
		}
		if price.CacheWritePerMillion != 0 {
			entry["cache_write_per_million"] = price.CacheWritePerMillion
		}
		canonical[model] = entry
	}
	body, _ := json.Marshal(canonical)
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum)
}

func comparePricingTables(current, latest map[string]Pricing) (added, removed, changed int) {
	for key, now := range latest {
		prev, ok := current[key]
		if !ok {
			added++
			continue
		}
		if prev != now {
			changed++
		}
	}
	for key := range current {
		if _, ok := latest[key]; !ok {
			removed++
		}
	}
	return added, removed, changed
}
