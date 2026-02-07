package ingest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/strongdm/kilroy/internal/attractor/engine"
)

// Options configures an ingestion run.
type Options struct {
	Requirements string // The English requirements text.
	SkillPath    string // Path to the SKILL.md file.
	Model        string // LLM model ID.
	RepoPath     string // Repository root (working directory for claude).
	Validate     bool   // Whether to validate the .dot output.
	MaxTurns     int    // Max turns for claude (default 3).
}

// Result contains the output of an ingestion run.
type Result struct {
	DotContent string   // The extracted .dot file content.
	RawOutput  string   // The full raw output from Claude Code.
	Warnings   []string // Any validation warnings.
}

func buildCLIArgs(opts Options) (string, []string) {
	exe := envOr("KILROY_CLAUDE_PATH", "claude")
	maxTurns := opts.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 3
	}

	args := []string{
		"-p",
		"--output-format", "text",
		"--model", opts.Model,
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
	}

	if opts.SkillPath != "" {
		args = append(args, "--append-system-prompt-file", opts.SkillPath)
	}

	// The requirements are the prompt — appended last.
	args = append(args, opts.Requirements)

	return exe, args
}

// Run executes the ingestion: invokes Claude Code with the skill and requirements,
// extracts the .dot content, and optionally validates it.
func Run(ctx context.Context, opts Options) (*Result, error) {
	// Verify skill file exists.
	if _, err := os.Stat(opts.SkillPath); err != nil {
		return nil, fmt.Errorf("skill file not found: %s: %w", opts.SkillPath, err)
	}

	exe, args := buildCLIArgs(opts)

	cmd := exec.CommandContext(ctx, exe, args...)
	if opts.RepoPath != "" {
		cmd.Dir = opts.RepoPath
	}
	cmd.Stdin = strings.NewReader("")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	rawOutput := stdout.String()
	if err != nil {
		return nil, fmt.Errorf("claude invocation failed (exit %v): %s\nstderr: %s",
			err, truncateStr(rawOutput, 500), truncateStr(stderr.String(), 500))
	}

	// Extract the digraph from the output.
	dotContent, err := ExtractDigraph(rawOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to extract digraph from output: %w\nraw output (first 1000 chars): %s",
			err, truncateStr(rawOutput, 1000))
	}

	result := &Result{
		DotContent: dotContent,
		RawOutput:  rawOutput,
	}

	// Optionally validate.
	if opts.Validate {
		_, diags, err := engine.Prepare([]byte(dotContent))
		if err != nil {
			return result, fmt.Errorf("generated .dot failed validation: %w", err)
		}
		for _, d := range diags {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s (%s)", d.Severity, d.Message, d.Rule))
		}
	}

	return result, nil
}

func envOr(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
