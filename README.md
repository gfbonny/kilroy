# Kilroy

Kilroy is a local-first "software factory" CLI that turns English requirements into an **Attractor** pipeline (a directed graph in Graphviz DOT), then executes that pipeline node-by-node with tool-using coding agents in an isolated git worktree.

## What Are Attractors?

An **Attractor** is a DOT-based pipeline runner for multi-stage AI workflows:

- A pipeline is a `digraph` written in Graphviz DOT syntax.
- **Nodes** represent stages (LLM/coding-agent tasks, human gates, tool steps, conditionals, parallel fan-out, etc.).
- **Edges** represent control flow, including conditions and retry loops.
- The engine checkpoints progress and (in Kilroy) commits after each node so runs can be resumed and audited.

Spec references:
- `docs/strongdm/attractor/attractor-spec.md` (graph DSL + engine semantics)
- `docs/strongdm/attractor/coding-agent-loop-spec.md` (tool-using coding agent loop)
- `docs/strongdm/attractor/unified-llm-spec.md` (provider-neutral LLM client)
- `docs/strongdm/attractor/kilroy-metaspec.md` (this repo's pinned decisions)

## StrongDM Links

The Attractor + CXDB specs vendored into `docs/strongdm/` originate from StrongDM:

- StrongDM: https://strongdm.com/
- CXDB: https://github.com/strongdm/cxdb
- In-repo specs: `docs/strongdm/attractor/`

## CLI Quickstart

```bash
go build -o kilroy ./cmd/kilroy

# Turn requirements into a pipeline graph
./kilroy attractor ingest -o pipeline.dot "Build a Go CLI link checker with robots.txt support"

# Validate DOT structure/syntax
./kilroy attractor validate --graph pipeline.dot

# Execute the pipeline (requires a run config; see metaspec)
./kilroy attractor run --graph pipeline.dot --config run.yaml
```

For the `run.yaml` schema, CXDB requirements, and execution model, read `docs/strongdm/attractor/kilroy-metaspec.md`.

## Skills: "Use The Skill To Use This Repo"

This repo ships Codex/Claude-style skill documents under `skills/`:

- `skills/using-kilroy/SKILL.md`: how to operate the `kilroy attractor` workflow (ingest/validate/run/resume).
- `skills/english-to-dotfile/SKILL.md`: how to turn requirements into a valid `.dot` pipeline (used by `kilroy attractor ingest`).

To use them with a coding agent that supports `SKILL.md` documents:

1. Add the skill(s) to your agent's skill search path (for example, copy or symlink `skills/using-kilroy/` into your agent's skills directory).
2. In your prompt, tell the agent to use the `using-kilroy` skill when you want it to run Kilroy commands (and `english-to-dotfile` when you want a pipeline).

If you're using Kilroy's ingestor directly, it will auto-detect `skills/english-to-dotfile/SKILL.md` unless you pass `--skill`.
