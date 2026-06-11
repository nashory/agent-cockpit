#!/usr/bin/env python3
"""Generate compact, human-readable release notes from commit subjects."""

from __future__ import annotations

import argparse
import re
import subprocess
import sys


INTERNAL_PATTERNS = (
    "numeric pre-1.0 release cadence",
    "release cadence",
)

LABELS = {
    "cache": "Cache",
    "ci": "CI",
    "cli": "CLI",
    "config": "Config",
    "docs": "Docs",
    "perf": "Performance",
    "pricing": "Pricing",
    "report": "Reports",
    "serve": "Local API",
    "source": "Sources",
    "statusline": "Statusline",
    "test": "Tests",
    "tui": "TUI",
}

TERMS = {
    "api": "API",
    "csv": "CSV",
    "json": "JSON",
    "svg": "SVG",
    "ui": "UI",
    "tui": "TUI",
    "cli": "CLI",
    "ci": "CI",
    "amp": "Amp",
    "claude": "Claude",
    "copilot": "Copilot",
    "gemini": "Gemini",
    "kimi": "Kimi",
    "opencode": "OpenCode",
    "waybar": "Waybar",
    "sketchybar": "SketchyBar",
    "windows": "Windows",
    "tmux": "tmux",
    "ccusage": "ccusage",
    "localhost": "localhost",
}


def git(*args: str) -> str:
    return subprocess.check_output(["git", *args], text=True).strip()


def previous_tag(target: str) -> str:
    try:
        return git("describe", "--tags", "--abbrev=0", f"{target}^")
    except subprocess.CalledProcessError:
        return ""


def commit_subjects(target: str) -> list[str]:
    prev = previous_tag(target)
    range_spec = f"{prev}..{target}" if prev else target
    out = git("log", "--reverse", "--format=%s", range_spec)
    return [line.strip() for line in out.splitlines() if line.strip()]


def normalize_term(word: str) -> str:
    lower = word.lower()
    if lower in TERMS:
        return TERMS[lower]
    return word


def sentence_case(text: str) -> str:
    words = [normalize_term(word) for word in text.split()]
    if not words:
        return ""
    words[0] = words[0][:1].upper() + words[0][1:]
    return " ".join(words)


def humanize(subject: str) -> str:
    subject = re.sub(r"\s+\(#\d+\)$", "", subject).strip()
    match = re.match(r"^([a-z0-9_-]+)(?:\([^)]+\))?:\s+(.+)$", subject)
    if match:
        label = LABELS.get(match.group(1), match.group(1).replace("-", " ").title())
        text = sentence_case(match.group(2))
        return f"{label}: {text}."
    return sentence_case(subject).rstrip(".") + "."


def release_notes(target: str) -> str:
    bullets: list[str] = []
    seen: set[str] = set()
    for subject in commit_subjects(target):
        lowered = subject.lower()
        if any(pattern in lowered for pattern in INTERNAL_PATTERNS):
            continue
        bullet = humanize(subject)
        if bullet not in seen:
            seen.add(bullet)
            bullets.append(bullet)
    if not bullets:
        bullets = ["Maintenance updates."]
    return "\n".join(["## Changes", "", *[f"- {bullet}" for bullet in bullets], ""])


def check_notes(notes: str) -> int:
    lines = notes.splitlines()
    bullets = [line for line in lines if line.startswith("- ")]
    if not bullets:
        print("release notes must contain at least one bullet", file=sys.stderr)
        return 1
    for line in bullets:
        if len(line) > 160:
            print(f"release note bullet is too long: {line}", file=sys.stderr)
            return 1
        if any(pattern in line.lower() for pattern in INTERNAL_PATTERNS):
            print(f"release note leaks internal rule: {line}", file=sys.stderr)
            return 1
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("target", help="git tag, commit, or HEAD")
    parser.add_argument("--check", action="store_true", help="validate generated note shape")
    args = parser.parse_args()

    notes = release_notes(args.target)
    if args.check:
        return check_notes(notes)
    print(notes, end="")
    return 0


if __name__ == "__main__":
    sys.exit(main())
