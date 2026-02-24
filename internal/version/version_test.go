package version

import (
	"regexp"
	"testing"
)

func TestVersionIsSemver(t *testing.T) {
	// Version must be a valid semver string. Accepts MAJOR.MINOR.PATCH with
	// optional prerelease suffix (e.g. 1.2.3-rc.1) and build metadata
	// (e.g. 1.2.3+meta). goreleaser uses prerelease: auto, so prerelease
	// tags like v0.2.0-rc.1 are valid.
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	if !semver.MatchString(Version) {
		t.Fatalf("Version=%q does not match semver pattern", Version)
	}
}
