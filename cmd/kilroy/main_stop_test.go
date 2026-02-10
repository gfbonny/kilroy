package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAttractorStop_KillsVerifiedAttractorProcessFromRunPID(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("requires sleep binary")
	}
	bin := buildKilroyBinary(t)
	logs := t.TempDir()

	proc := exec.Command("bash", "-lc", "sleep 60", "kilroy", "attractor", "run", "--logs-root", logs)
	if err := proc.Start(); err != nil {
		t.Fatalf("start mock attractor process: %v", err)
	}
	t.Cleanup(func() {
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
	})
	pid := proc.Process.Pid
	if pid <= 0 {
		t.Fatalf("invalid pid: %d", pid)
	}
	_ = os.WriteFile(filepath.Join(logs, "run.pid"), []byte(strconv.Itoa(pid)), 0o644)

	out, err := exec.Command(bin, "attractor", "stop", "--logs-root", logs, "--grace-ms", "500", "--force").CombinedOutput()
	if err != nil {
		t.Fatalf("stop failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "stopped=") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAttractorStop_RefusesAttractorProcessWithoutIdentityFlags(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("requires sleep binary")
	}
	bin := buildKilroyBinary(t)
	logs := t.TempDir()

	proc := exec.Command("bash", "-lc", "sleep 60", "kilroy", "attractor", "run")
	if err := proc.Start(); err != nil {
		t.Fatalf("start mock attractor process: %v", err)
	}
	t.Cleanup(func() {
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
	})
	pid := proc.Process.Pid
	if pid <= 0 {
		t.Fatalf("invalid pid: %d", pid)
	}
	_ = os.WriteFile(filepath.Join(logs, "run.pid"), []byte(strconv.Itoa(pid)), 0o644)

	out, err := exec.Command(bin, "attractor", "stop", "--logs-root", logs).CombinedOutput()
	if err == nil {
		t.Fatalf("expected stop to fail when identity flags are missing; output=%s", out)
	}
	if !strings.Contains(string(out), "no --logs-root/--run-id") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAttractorStop_RefusesPIDWithoutAttractorIdentity(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("requires sleep binary")
	}
	bin := buildKilroyBinary(t)
	logs := t.TempDir()

	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
	})
	_ = os.WriteFile(filepath.Join(logs, "run.pid"), []byte(strconv.Itoa(proc.Process.Pid)), 0o644)

	out, err := exec.Command(bin, "attractor", "stop", "--logs-root", logs).CombinedOutput()
	if err == nil {
		t.Fatalf("expected stop to fail for non-attractor pid; output=%s", out)
	}
	if !strings.Contains(string(out), "refusing to signal pid") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAttractorStop_RefusesWhenRunIsTerminal(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("requires sleep binary")
	}
	bin := buildKilroyBinary(t)
	logs := t.TempDir()

	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
	})
	_ = os.WriteFile(filepath.Join(logs, "run.pid"), []byte(strconv.Itoa(proc.Process.Pid)), 0o644)
	_ = os.WriteFile(filepath.Join(logs, "final.json"), []byte(`{"status":"success","run_id":"r1"}`), 0o644)

	out, err := exec.Command(bin, "attractor", "stop", "--logs-root", logs).CombinedOutput()
	if err == nil {
		t.Fatalf("expected stop to fail for terminal run; output=%s", out)
	}
	if !strings.Contains(string(out), `run state is "success"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAttractorStop_ErrorsWhenNoPID(t *testing.T) {
	bin := buildKilroyBinary(t)
	logs := t.TempDir()
	out, err := exec.Command(bin, "attractor", "stop", "--logs-root", logs).CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit; output=%s", out)
	}
}
