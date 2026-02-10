package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/runstate"
)

func attractorStop(args []string) {
	os.Exit(runAttractorStop(args, os.Stdout, os.Stderr))
}

func runAttractorStop(args []string, stdout io.Writer, stderr io.Writer) int {
	var logsRoot string
	grace := 5 * time.Second
	force := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--logs-root":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "--logs-root requires a value")
				return 1
			}
			logsRoot = args[i]
		case "--grace-ms":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "--grace-ms requires a value")
				return 1
			}
			ms, err := strconv.Atoi(args[i])
			if err != nil || ms < 0 {
				fmt.Fprintf(stderr, "invalid --grace-ms value: %q\n", args[i])
				return 1
			}
			grace = time.Duration(ms) * time.Millisecond
		case "--force":
			force = true
		default:
			fmt.Fprintf(stderr, "unknown arg: %s\n", args[i])
			return 1
		}
	}

	if logsRoot == "" {
		fmt.Fprintln(stderr, "--logs-root is required")
		return 1
	}

	snapshot, err := runstate.LoadSnapshot(logsRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if snapshot.State != runstate.StateRunning {
		fmt.Fprintf(stderr, "run state is %q (expected %q); refusing to stop\n", snapshot.State, runstate.StateRunning)
		return 1
	}
	if snapshot.PID <= 0 {
		fmt.Fprintln(stderr, "run pid is not available (run.pid missing or invalid)")
		return 1
	}
	if !snapshot.PIDAlive {
		fmt.Fprintf(stderr, "pid %d is not running\n", snapshot.PID)
		return 1
	}
	if err := verifyAttractorRunPID(snapshot.PID, logsRoot, snapshot.RunID); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	proc, err := os.FindProcess(snapshot.PID)
	if err != nil {
		fmt.Fprintf(stderr, "find pid %d: %v\n", snapshot.PID, err)
		return 1
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		fmt.Fprintf(stderr, "send SIGTERM to pid %d: %v\n", snapshot.PID, err)
		return 1
	}

	if waitForPIDExit(snapshot.PID, grace) {
		fmt.Fprintf(stdout, "pid=%d\nstopped=graceful\n", snapshot.PID)
		return 0
	}

	if !force {
		fmt.Fprintf(stderr, "pid %d did not exit within %s\n", snapshot.PID, grace)
		return 1
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		fmt.Fprintf(stderr, "send SIGKILL to pid %d: %v\n", snapshot.PID, err)
		return 1
	}
	forceWait := grace
	if forceWait < time.Second {
		forceWait = time.Second
	}
	if !waitForPIDExit(snapshot.PID, forceWait) {
		fmt.Fprintf(stderr, "pid %d did not exit after SIGKILL\n", snapshot.PID)
		return 1
	}
	fmt.Fprintf(stdout, "pid=%d\nstopped=forced\n", snapshot.PID)
	return 0
}

func waitForPIDExit(pid int, grace time.Duration) bool {
	if !pidRunning(pid) {
		return true
	}
	deadline := time.Now().Add(grace)
	poll := adaptiveGracePoll(grace)
	for time.Now().Before(deadline) {
		time.Sleep(poll)
		if !pidRunning(pid) {
			return true
		}
	}
	return !pidRunning(pid)
}

func adaptiveGracePoll(grace time.Duration) time.Duration {
	poll := grace / 5
	if poll < 10*time.Millisecond {
		poll = 10 * time.Millisecond
	}
	if poll > 100*time.Millisecond {
		poll = 100 * time.Millisecond
	}
	return poll
}

func pidRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pidZombie(pid) {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}

func pidZombie(pid int) bool {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	b, err := os.ReadFile(statPath)
	if err != nil {
		return false
	}
	line := string(b)
	closeIdx := strings.LastIndexByte(line, ')')
	if closeIdx < 0 || closeIdx+2 >= len(line) {
		return false
	}
	state := line[closeIdx+2]
	return state == 'Z' || state == 'X'
}

func verifyAttractorRunPID(pid int, logsRoot string, runID string) error {
	args, err := readPIDCmdline(pid)
	if err != nil {
		return fmt.Errorf("refusing to signal pid %d: cannot read process command line: %w", pid, err)
	}
	if len(args) == 0 {
		return fmt.Errorf("refusing to signal pid %d: empty process command line", pid)
	}

	attractorIdx := -1
	for i, arg := range args {
		if strings.TrimSpace(arg) == "attractor" {
			attractorIdx = i
			break
		}
	}
	if attractorIdx < 0 || attractorIdx+1 >= len(args) {
		return fmt.Errorf("refusing to signal pid %d: process is not an attractor run/resume command", pid)
	}
	sub := strings.TrimSpace(args[attractorIdx+1])
	if sub != "run" && sub != "resume" {
		return fmt.Errorf("refusing to signal pid %d: process is attractor %q, not run/resume", pid, sub)
	}

	if pidLogsRoot, ok := cmdlineLogsRoot(args); ok {
		if !samePath(pidLogsRoot, logsRoot) {
			return fmt.Errorf("refusing to signal pid %d: --logs-root mismatch (pid=%q requested=%q)", pid, pidLogsRoot, logsRoot)
		}
		return nil
	}

	if pidRunID, ok := cmdlineRunID(args); ok && strings.TrimSpace(runID) != "" {
		if strings.TrimSpace(pidRunID) != strings.TrimSpace(runID) {
			return fmt.Errorf("refusing to signal pid %d: --run-id mismatch (pid=%q snapshot=%q)", pid, pidRunID, runID)
		}
		return nil
	}
	return fmt.Errorf("refusing to signal pid %d: process command line has no --logs-root/--run-id", pid)
}

func readPIDCmdline(pid int) ([]string, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(b), "\x00")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func cmdlineLogsRoot(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--logs-root" && i+1 < len(args):
			return strings.TrimSpace(args[i+1]), true
		case strings.HasPrefix(args[i], "--logs-root="):
			return strings.TrimSpace(strings.TrimPrefix(args[i], "--logs-root=")), true
		}
	}
	return "", false
}

func cmdlineRunID(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--run-id" && i+1 < len(args):
			return strings.TrimSpace(args[i+1]), true
		case strings.HasPrefix(args[i], "--run-id="):
			return strings.TrimSpace(strings.TrimPrefix(args[i], "--run-id=")), true
		}
	}
	return "", false
}

func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}
