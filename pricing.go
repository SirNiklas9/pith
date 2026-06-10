package pith

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ModelPrice is one model's current rates in dollars per token, cached from a
// public catalog. The dry-run reads ONLY the cache, so previews stay offline
// and keyless; refreshing rates is an explicit `pith price` call.
type ModelPrice struct {
	In        float64   `json:"in"`  // $ per input token
	Out       float64   `json:"out"` // $ per output token
	FetchedAt time.Time `json:"fetched_at"`
}

// pricesPath is the rate cache, next to config.json in the user config dir.
func pricesPath() (string, error) {
	cfg, err := ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfg), "prices.json"), nil
}

// CachedPrice returns a model's cached rates, if `pith price` has fetched them.
func CachedPrice(model string) (ModelPrice, bool) {
	path, err := pricesPath()
	if err != nil || model == "" {
		return ModelPrice{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ModelPrice{}, false
	}
	prices := map[string]ModelPrice{}
	if json.Unmarshal(b, &prices) != nil {
		return ModelPrice{}, false
	}
	p, ok := prices[model]
	return p, ok
}

// FetchPrice looks a model up in OpenRouter's public model catalog (no key
// needed — it lists current rates for every major vendor's models) and caches
// the result for offline use. Matching tries the exact id, then a /suffix
// match so bare names like "gpt-4o-mini" find "openai/gpt-4o-mini".
func FetchPrice(model string) (ModelPrice, error) {
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return ModelPrice{}, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ModelPrice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ModelPrice{}, fmt.Errorf("price catalog: HTTP %d", resp.StatusCode)
	}

	var catalog struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return ModelPrice{}, err
	}

	for _, m := range catalog.Data {
		if m.ID != model && !strings.HasSuffix(m.ID, "/"+model) {
			continue
		}
		in, e1 := strconv.ParseFloat(m.Pricing.Prompt, 64)
		out, e2 := strconv.ParseFloat(m.Pricing.Completion, 64)
		if e1 != nil || e2 != nil {
			return ModelPrice{}, fmt.Errorf("catalog has no usable rates for %s", m.ID)
		}
		p := ModelPrice{In: in, Out: out, FetchedAt: time.Now()}
		savePrice(model, p)
		return p, nil
	}
	return ModelPrice{}, fmt.Errorf("model %q not in the catalog (try the full id, e.g. anthropic/claude-haiku-4.5)", model)
}

// savePrice merges one model's rates into the cache file (best-effort).
func savePrice(model string, p ModelPrice) {
	path, err := pricesPath()
	if err != nil {
		return
	}
	prices := map[string]ModelPrice{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &prices)
	}
	prices[model] = p
	if b, err := json.MarshalIndent(prices, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		_ = os.WriteFile(path, b, 0o600)
	}
}

// Dollars renders a small dollar amount with sensible precision.
func Dollars(d float64) string {
	switch {
	case d >= 0.01:
		return fmt.Sprintf("$%.2f", d)
	case d >= 0.0001:
		return fmt.Sprintf("$%.4f", d)
	default:
		return fmt.Sprintf("$%.6f", d)
	}
}
