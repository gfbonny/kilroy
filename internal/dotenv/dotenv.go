// Package dotenv loads KEY=VALUE pairs from a .env file into the process
// environment. It follows the standard dotenv conventions used by tools like
// Docker Compose, Vite, and direnv.
package dotenv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Load reads the file at path and sets each key that is not already present in
// the environment. If path does not exist, Load returns nil (silent ignore).
// An error is returned only for malformed lines or I/O failures on an existing file.
func Load(path string) error {
	return load(path, false)
}

// Overload reads the file at path and sets ALL keys, overwriting existing
// environment variables. If path does not exist, Overload returns nil.
func Overload(path string) error {
	return load(path, true)
}

func load(path string, overwrite bool) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	pairs, err := parse(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("dotenv %s: %w", path, err)
	}

	for _, kv := range pairs {
		if !overwrite {
			if _, exists := os.LookupEnv(kv[0]); exists {
				continue
			}
		}
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			return fmt.Errorf("dotenv setenv %s: %w", kv[0], err)
		}
	}
	return nil
}

// parse reads lines from r and returns [key, value] pairs.
func parse(r io.Reader) ([][2]string, error) {
	var pairs [][2]string
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comment lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip optional "export " prefix.
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		eqIdx := strings.IndexByte(line, '=')
		if eqIdx < 0 {
			return nil, fmt.Errorf("line %d: no '=' found: %q", lineNum, line)
		}

		key := strings.TrimSpace(line[:eqIdx])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		raw := line[eqIdx+1:]
		value, err := parseValue(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		pairs = append(pairs, [2]string{key, value})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return pairs, nil
}

// parseValue handles quoted and unquoted values with optional inline comments.
func parseValue(raw string) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	// Double-quoted value: preserve whitespace, no interpolation.
	if raw[0] == '"' {
		end := strings.Index(raw[1:], "\"")
		if end < 0 {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		return raw[1 : end+1], nil
	}

	// Single-quoted value: literal, no escape processing.
	if raw[0] == '\'' {
		end := strings.Index(raw[1:], "'")
		if end < 0 {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		return raw[1 : end+1], nil
	}

	// Unquoted value: strip inline comment and trim whitespace.
	if idx := strings.IndexByte(raw, '#'); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw), nil
}
