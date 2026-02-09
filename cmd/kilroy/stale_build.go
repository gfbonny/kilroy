package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

type staleBuildStatus struct {
	Stale         bool
	BinaryPath    string
	RepoRoot      string
	BuiltRevision string
	HeadRevision  string
}

// embeddedBuildRevision can be set at build time:
//
//	go build -ldflags "-X main.embeddedBuildRevision=<sha>"
//
// When unset, runtime build info is used.
var embeddedBuildRevision string

func ensureFreshKilroyBuild(confirmStaleBuild bool) error {
	status, ok := detectStaleKilroyBuild()
	if !ok || !status.Stale {
		return nil
	}
	warning := staleBuildWarning(status)
	if !confirmStaleBuild {
		return fmt.Errorf("%s\nrefusing to run a stale build without --confirm-stale-build", warning)
	}
	fmt.Fprintln(os.Stderr, warning)
	fmt.Fprintln(os.Stderr, "proceeding because --confirm-stale-build was provided")
	return nil
}

func detectStaleKilroyBuild() (staleBuildStatus, bool) {
	rev, ok := binaryVCSRevision()
	if !ok {
		return staleBuildStatus{}, false
	}
	exePath, err := os.Executable()
	if err != nil {
		return staleBuildStatus{}, false
	}
	if evalPath, evalErr := filepath.EvalSymlinks(exePath); evalErr == nil {
		exePath = evalPath
	}
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return staleBuildStatus{}, false
	}
	repoRoot, ok := gitTopLevel(filepath.Dir(exePath))
	if !ok {
		return staleBuildStatus{}, false
	}
	head, ok := gitHEADRevision(repoRoot)
	if !ok {
		return staleBuildStatus{}, false
	}
	status := staleBuildStatus{
		BinaryPath:    exePath,
		RepoRoot:      repoRoot,
		BuiltRevision: rev,
		HeadRevision:  head,
	}
	status.Stale = !sameRevision(status.BuiltRevision, status.HeadRevision)
	return status, true
}

func binaryVCSRevision() (string, bool) {
	if rev := strings.TrimSpace(embeddedBuildRevision); rev != "" {
		return rev, true
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "", false
	}
	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}
		rev := strings.TrimSpace(s.Value)
		if rev == "" {
			return "", false
		}
		return rev, true
	}
	return "", false
}

func gitTopLevel(dir string) (string, bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", false
	}
	return root, true
}

func gitHEADRevision(repoRoot string) (string, bool) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", false
	}
	head := strings.TrimSpace(string(out))
	if head == "" {
		return "", false
	}
	return head, true
}

func sameRevision(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func staleBuildWarning(status staleBuildStatus) string {
	return strings.Join([]string{
		"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
		"WARNING: STALE KILROY BUILD DETECTED",
		fmt.Sprintf("binary: %s", status.BinaryPath),
		fmt.Sprintf("repo_root: %s", status.RepoRoot),
		fmt.Sprintf("built_revision: %s", shortRevision(status.BuiltRevision)),
		fmt.Sprintf("repo_head: %s", shortRevision(status.HeadRevision)),
		"rebuild with: go build -o ./kilroy ./cmd/kilroy",
		"then rerun; or pass --confirm-stale-build to continue anyway",
		"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
	}, "\n")
}

func shortRevision(rev string) string {
	rev = strings.TrimSpace(rev)
	if len(rev) <= 12 {
		return rev
	}
	return rev[:12]
}
