# CLAUDE.md

## Overview

cc-journal is a Go CLI tool that auto-summarizes Claude Code sessions into a developer journal. It parses JSONL transcripts, generates AI summaries via the Anthropic API, and serves a web dashboard.

## CLI Framework

Uses [urfave/cli v3](https://github.com/urfave/cli) for command/flag parsing and [usage](https://usage.jdx.dev) (KDL spec) for shell completions and CLI docs.

**Important:** The urfave command definitions in `main.go` and the usage spec in `cc-journal.usage.kdl` must stay in sync. When adding or changing a command or flag:

1. Update the urfave `cli.Command` in `main.go`
2. Update `cc-journal.usage.kdl` to match
3. Run `mise run render` to regenerate completions and docs

## Development

```bash
mise run dev          # Build and install dev version locally
mise run serve        # Build and start dev server
mise run render       # Regenerate completions and docs from usage spec
mise run check        # Build and vet
```

## Pre-commit Hooks

Uses [hk](https://hk.jdx.dev) for pre-commit hooks: go-fmt, go-vet, golangci-lint. CI runs the same checks via `hk run check --all`.

## Key Conventions

- YAML files use `.yaml` extension (not `.yml`)
- Journal entries stored as markdown in `~/claude-journal/YYYY-MM-DD.md`
- Token data stored in `<code>tokens:...</code>` inside `<details>` blocks
- Templates are embedded via `//go:embed` with optional override directory
- Config loaded from `~/.config/cc-journal/config.yaml`
