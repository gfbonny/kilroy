package engine

import (
	"reflect"
	"testing"
)

func TestStripSandboxFlag(t *testing.T) {
	in := []string{"exec", "--json", "--sandbox", "workspace-write", "-m", "o3"}
	got := stripSandboxFlag(in)
	want := []string{"exec", "--json", "-m", "o3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripSandboxFlag() = %v, want %v", got, want)
	}
}

func TestStripSandboxFlag_NoSandbox(t *testing.T) {
	in := []string{"exec", "--json", "-m", "o3"}
	got := stripSandboxFlag(in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("stripSandboxFlag() = %v, want %v (unchanged)", got, in)
	}
}

func TestStripSandboxFlag_SandboxAtEnd(t *testing.T) {
	// --sandbox as last arg with no value — should be preserved (not a valid pair).
	in := []string{"exec", "--json", "--sandbox"}
	got := stripSandboxFlag(in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("stripSandboxFlag() = %v, want %v (unchanged)", got, in)
	}
}
