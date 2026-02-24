package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func resolveDetachedPaths(graphPath, configPath, logsRoot string) (string, string, string, error) {
	graphPath = strings.TrimSpace(graphPath)
	configPath = strings.TrimSpace(configPath)
	logsRoot = strings.TrimSpace(logsRoot)
	if graphPath == "" {
		return "", "", "", fmt.Errorf("graph path is required")
	}
	if configPath == "" {
		return "", "", "", fmt.Errorf("config path is required")
	}
	if logsRoot == "" {
		return "", "", "", fmt.Errorf("logs root is required")
	}

	absGraph, err := filepath.Abs(graphPath)
	if err != nil {
		return "", "", "", err
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return "", "", "", err
	}
	absLogs, err := filepath.Abs(logsRoot)
	if err != nil {
		return "", "", "", err
	}
	return absGraph, absConfig, absLogs, nil
}
