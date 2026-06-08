#!/usr/bin/env python3
"""Distill LiteLLM's pricing database into a compact, normalized table.

agent-cockpit stays 100% local at runtime, so prices are vendored into the
binary (see internal/usage/pricing_data.json + pricing_embed.go) rather than
fetched. Re-run this on release to refresh:

    python3 scripts/gen-pricing.py

Source: https://github.com/BerriAI/litellm (model_prices_and_context_window.json)
It keeps only the providers agent-cockpit reads (Anthropic / OpenAI / Gemini),
strips provider and region prefixes so log model names match, converts per-token
costs to per-million, and prefers the plainest key on collisions.
"""
import json
import re
import sys
import urllib.request

SRC = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
OUT = "internal/usage/pricing_data.json"

# Region / platform prefixes that wrap the same underlying model.
PREFIX = re.compile(
    r"^(us|eu|apac|au|ca|sa|global|anthropic|bedrock|vertex_ai|vertex|"
    r"azure_ai|azure|openai|gemini|google|fireworks_ai|openrouter|"
    r"converse|invoke)[./]"
)
SUFFIX = re.compile(r"(-v\d+:\d+|:\d+|-v\d+|@\d{8})$")


def normalize(key: str) -> str:
    k = key.lower()
    if "/" in k:
        k = k.split("/")[-1]
    prev = None
    while prev != k:
        prev = k
        k = PREFIX.sub("", k)
    prev = None
    while prev != k:
        prev = k
        k = SUFFIX.sub("", k)
    return k


def family(key: str, prov: str) -> bool:
    k = key.lower()
    if prov in ("anthropic", "openai", "gemini", "vertex_ai-language-models"):
        return True
    return bool(re.search(r"(claude|gpt-|gpt4|o1|o3|o4|codex|gemini)", k))


def main() -> int:
    raw = urllib.request.urlopen(SRC, timeout=30).read()
    data = json.loads(raw)

    out: dict[str, dict] = {}
    # Shorter keys (plainer, no region markup) win on collision.
    for key in sorted(data, key=len):
        v = data[key]
        if not isinstance(v, dict):
            continue
        if v.get("mode") not in (None, "chat", "completion"):
            continue
        inp = v.get("input_cost_per_token")
        outp = v.get("output_cost_per_token")
        if inp is None or outp is None:
            continue
        if not family(key, v.get("litellm_provider", "")):
            continue
        name = normalize(key)
        if not name or name in out:
            continue
        entry = {
            "input_per_million": round(inp * 1e6, 6),
            "output_per_million": round(outp * 1e6, 6),
        }
        if v.get("cache_read_input_token_cost") is not None:
            entry["cache_read_per_million"] = round(v["cache_read_input_token_cost"] * 1e6, 6)
        if v.get("cache_creation_input_token_cost") is not None:
            entry["cache_write_per_million"] = round(v["cache_creation_input_token_cost"] * 1e6, 6)
        out[name] = entry

    with open(OUT, "w") as f:
        json.dump(out, f, indent=0, sort_keys=True)
        f.write("\n")
    print(f"wrote {len(out)} models to {OUT}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
