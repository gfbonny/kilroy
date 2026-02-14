package version

import (
	"regexp"
	"testing"
)

func TestVersionIsSemver(t *testing.T) {
	// Version must be a valid semver string (MAJOR.MINOR.PATCH).
	// goreleaser also injects it from the git tag at build time via ldflags,
	// but source builds must have a real version for the auto-updater.
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !semver.MatchString(Version) {
		t.Fatalf("Version=%q does not match semver pattern MAJOR.MINOR.PATCH", Version)
	}
}
