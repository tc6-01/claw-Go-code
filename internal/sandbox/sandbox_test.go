package sandbox

import (
	"testing"
)

func TestDisabledSandbox(t *testing.T) {
	s := New(Config{Enabled: false})

	if err := s.ValidatePath("/etc/passwd"); err != nil {
		t.Fatalf("disabled sandbox should allow all: %v", err)
	}
	if err := s.ValidateCommand("rm -rf /"); err != nil {
		t.Fatalf("disabled sandbox should allow commands: %v", err)
	}
	if s.IsEnabled() {
		t.Fatal("should not be enabled")
	}
}

func TestEnabledSandboxPathValidation(t *testing.T) {
	s := New(Config{
		Enabled:     true,
		RootDir:     "/workspace",
		AllowedDirs: []string{"/tmp/extra"},
	})

	if err := s.ValidatePath("/workspace/src/main.go"); err != nil {
		t.Fatalf("should allow path under root: %v", err)
	}
	if err := s.ValidatePath("/tmp/extra/file.txt"); err != nil {
		t.Fatalf("should allow path under allowed dir: %v", err)
	}
	if err := s.ValidatePath("/etc/passwd"); err == nil {
		t.Fatal("should deny path outside sandbox")
	}
}

func TestDenyExec(t *testing.T) {
	s := New(Config{
		Enabled:  true,
		RootDir:  "/workspace",
		DenyExec: true,
	})

	if err := s.ValidateCommand("ls"); err == nil {
		t.Fatal("should deny exec when DenyExec is true")
	}
	if !s.DenyExec() {
		t.Fatal("DenyExec should return true")
	}
}

func TestAllowExec(t *testing.T) {
	s := New(Config{
		Enabled:  true,
		RootDir:  "/workspace",
		DenyExec: false,
	})

	if err := s.ValidateCommand("ls"); err != nil {
		t.Fatalf("should allow exec: %v", err)
	}
}
