---
name: using-kilroy
description: Use when operating the Kilroy CLI to build software from English requirements via Attractor pipelines, or when configuring, running, validating, resuming, or ingesting pipelines.
---

# Using Kilroy

Kilroy builds software by converting English requirements into DOT pipeline graphs, then executing those graphs node-by-node with LLM agents. Each node invokes a coding agent that reads, writes, and tests code in an isolated git worktree.

## Workflow

```
English requirements
    |
    v
kilroy attractor ingest → pipeline.dot
    |
    v
kilroy attractor validate → check syntax
    |
    v
Create run.yaml config
    |
    v
kilroy attractor run → git branch with working code
```

## Commands

### Ingest: English to Pipeline

```bash
kilroy attractor ingest [flags] <requirements>
```

| Flag | Default | Purpose |
|------|---------|---------|
| `-o, --output <path>` | stdout | Write .dot file |
| `--model <model>` | `claude-sonnet-4-5` | LLM for generation |
| `--skill <path>` | auto-detect | Skill file (auto-finds `skills/english-to-dotfile/SKILL.md`) |
| `--repo <path>` | cwd | Repository root |
| `--no-validate` | validate | Skip validation |

```bash
# From English to validated pipeline
kilroy attractor ingest -o pipeline.dot "Build a Go CLI link checker with robots.txt support"

# Reference an existing spec
kilroy attractor ingest -o pipeline.dot "Build DTTF per specs/dttf-v1.md"

# Use Opus for complex requirements
kilroy attractor ingest --model claude-opus-4-6 -o pipeline.dot "Build a distributed task queue"
```

The ingest command invokes Claude Code with the `english-to-dotfile` skill. For DOT file structure, node patterns, and prompt authoring, see the `english-to-dotfile` skill.

### Validate: Check Pipeline Syntax

```bash
kilroy attractor validate --graph pipeline.dot
```

Validates DOT syntax, node shapes, edge conditions, graph structure (one start, one exit, all nodes reachable). Exit 0 = valid, exit 1 = invalid.

### Run: Execute Pipeline

```bash
kilroy attractor run --graph pipeline.dot --config run.yaml [--run-id <id>] [--logs-root <dir>]
```

| Flag | Default | Purpose |
|------|---------|---------|
| `--graph <file.dot>` | required | Pipeline to execute |
| `--config <run.yaml>` | required | Run configuration |
| `--run-id <id>` | auto (ULID) | Unique run identifier |
| `--logs-root <dir>` | `~/.local/state/kilroy/attractor/runs/<run_id>` | Logs and worktree location |

**Output** (key=value to stdout):
```
run_id=01JKXYZ...
logs_root=/home/user/.local/state/kilroy/attractor/runs/01JKXYZ...
worktree=/home/user/.local/state/kilroy/attractor/runs/01JKXYZ.../worktree
run_branch=attractor/run/01JKXYZ...
final_commit=abc123...
```

**What happens during a run:**
1. Creates git branch `attractor/run/<run_id>` at current HEAD
2. Creates isolated git worktree (all tracked files available including `skills/`)
3. Executes nodes in graph order, each via an LLM agent
4. Commits after every node (even if no changes)
5. Logs all artifacts to `{logs_root}/<node_id>/`
6. Records events to CXDB

**Exit codes:** 0 = success, 1 = failure

### Resume: Continue Interrupted Run

Three methods, all converge on the same internal logic:

```bash
# Method 1: From logs directory (most common)
kilroy attractor resume --logs-root <path>

# Method 2: From CXDB context
kilroy attractor resume --cxdb <http_url> --context-id <uuid>

# Method 3: From git branch
kilroy attractor resume --run-branch attractor/run/<id> [--repo <path>]
```

| Method | Use When |
|--------|----------|
| `--logs-root` | You have the logs directory path (printed by `run`) |
| `--cxdb` | Logs directory is lost but CXDB is running |
| `--run-branch` | Both logs and CXDB are unavailable; git branch exists |

Resume resets the worktree to the last checkpoint commit and continues from the next node.

## Configuration

### run.yaml (Minimal)

```yaml
version: 1

repo:
  path: /absolute/path/to/git/repo

cxdb:
  binary_addr: 127.0.0.1:9009
  http_base_url: http://127.0.0.1:9010

llm:
  providers:
    anthropic:
      backend: cli    # or api

modeldb:
  litellm_catalog_path: /path/to/model_prices_and_context_window.json
  litellm_catalog_update_policy: on_run_start

git:
  require_clean: true
  run_branch_prefix: attractor/run
  commit_per_node: true
```

### Backend Selection

Every provider used in the pipeline MUST have an explicit backend. No defaults.

| Backend | When to Use | Requirements |
|---------|-------------|--------------|
| `cli` | Full agent capabilities (file editing, tool use, multi-turn) | Provider CLI installed (`claude`, `codex`, `gemini`) |
| `api` | Direct API calls via Kilroy's built-in agent loop | API key in environment (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`) |

If a node uses `llm_provider: anthropic` but the config has no `anthropic` entry under `llm.providers`, the run fails immediately.

### Supported Providers

| Provider | CLI | API Key Env Var |
|----------|-----|-----------------|
| `anthropic` | `claude` | `ANTHROPIC_API_KEY` |
| `openai` | `codex` | `OPENAI_API_KEY` |
| `google` | `gemini` | `GEMINI_API_KEY` |

## Prerequisites Checklist

Before running a pipeline:

- [ ] **Build kilroy:** `go build -o kilroy ./cmd/kilroy` (or use existing binary)
- [ ] **Target repo is a git repo** with at least one commit
- [ ] **Clean working tree** (no uncommitted changes; `git.require_clean: true`)
- [ ] **CXDB running** at the configured `binary_addr` and `http_base_url`
- [ ] **Provider credentials** configured (CLI installed or API key in env)
- [ ] **Model catalog** available (auto-downloads with `on_run_start`, or provide path for `pinned`)

## Run Directory Layout

```
{logs_root}/
  graph.dot                  # Pipeline definition
  manifest.json              # Run metadata (repo, branch, CXDB context)
  checkpoint.json            # Execution state (current node, context, retries)
  final.json                 # Final status, commit SHA, CXDB IDs
  modeldb/
    litellm_catalog.json     # Per-run catalog snapshot
  worktree/                  # Isolated git worktree
  <node_id>/
    prompt.md                # Prompt sent to agent
    response.md              # Agent response
    status.json              # Node outcome (success/fail/retry)
```

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Missing backend config for a provider | Add provider under `llm.providers` with explicit `backend: api` or `backend: cli` |
| Dirty git repo | Commit or stash changes before running |
| CXDB not running | Start CXDB before `kilroy attractor run` |
| Wrong logs path on resume | Check original `run` output for `logs_root=` line |
| Relative path in `repo.path` | Use absolute path |
| Forgot `--graph` or `--config` | Both are required for `run` |
| Pipeline has no model_stylesheet | Every pipeline needs a model_stylesheet in graph attributes |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `KILROY_CLAUDE_PATH` | `claude` | Override path to Claude Code CLI |
| `XDG_STATE_HOME` | `~/.local/state` | Base for default logs location |

## Spec and Skill Locations

| What | Where |
|------|-------|
| Kilroy system specs | `docs/strongdm/attractor/` |
| Attractor DSL spec | `docs/strongdm/attractor/attractor-spec.md` |
| Metaspec (implementation decisions) | `docs/strongdm/attractor/kilroy-metaspec.md` |
| Ingestor spec | `docs/strongdm/attractor/ingestor-spec.md` |
| English-to-dotfile skill | `skills/english-to-dotfile/SKILL.md` |
| Product specs (things Kilroy builds) | `specs/` |
| Test coverage map | `docs/strongdm/attractor/test-coverage-map.md` |
