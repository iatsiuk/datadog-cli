package cmd

import (
	"testing"
)

func TestNewRootCommand_UseAndVersion(t *testing.T) {
	t.Parallel()
	cmd := NewRootCommand("1.2.3")
	if cmd.Use != "dd" {
		t.Errorf("Use = %q, want %q", cmd.Use, "dd")
	}
	if cmd.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", cmd.Version, "1.2.3")
	}
}

func TestNewRootCommand_JSONFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCommand("dev")
	f := cmd.PersistentFlags().Lookup("json")
	if f == nil {
		t.Fatal("--json persistent flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--json default = %q, want %q", f.DefValue, "false")
	}
}

func TestNewRootCommand_CompletionSubcommand(t *testing.T) {
	t.Parallel()
	root := NewRootCommand("dev")
	for _, sub := range root.Commands() {
		if sub.Name() == "completion" {
			return
		}
	}
	t.Error("completion subcommand not found")
}
