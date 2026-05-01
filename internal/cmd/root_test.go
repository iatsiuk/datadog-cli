package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCommand_UseAndVersion(t *testing.T) {
	t.Parallel()
	cmd := NewRootCommand("1.2.3")
	if cmd.Use != "datadog-cli" {
		t.Errorf("Use = %q, want %q", cmd.Use, "datadog-cli")
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

func TestNewRootCommand_SilencesUsageAndErrors(t *testing.T) {
	t.Parallel()
	root := NewRootCommand("dev")
	if !root.SilenceUsage {
		t.Error("SilenceUsage = false, want true")
	}
	if !root.SilenceErrors {
		t.Error("SilenceErrors = false, want true")
	}
}

func TestErrorJSONFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	WriteError(&buf, errors.New("boom"), true)

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%q", err, buf.String())
	}
	if got["error"] != "boom" {
		t.Errorf(`got["error"] = %q, want %q`, got["error"], "boom")
	}
	if strings.Contains(buf.String(), "Usage:") {
		t.Errorf("JSON output must not contain Usage block, got %q", buf.String())
	}
}

func TestErrorPlainFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	WriteError(&buf, errors.New("boom"), false)

	out := buf.String()
	if !strings.HasPrefix(out, "boom") {
		t.Errorf("plain output should start with the error string, got %q", out)
	}
	if strings.Contains(out, "Usage:") {
		t.Errorf("plain output must not contain Usage block, got %q", out)
	}
	var parsed map[string]string
	if json.Unmarshal([]byte(out), &parsed) == nil {
		t.Errorf("plain output must not be JSON-encoded, got %q", out)
	}
}

func TestParseJSONFlag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"--json"}, true},
		{[]string{"--json=true"}, true},
		{[]string{"--json=True"}, true},
		{[]string{"--json=TRUE"}, true},
		{[]string{"--json=1"}, true},
		{[]string{"--json=t"}, true},
		{[]string{"--json=T"}, true},
		{[]string{"--json=false"}, false},
		{[]string{"--json=0"}, false},
		{[]string{"--json=f"}, false},
		{[]string{"metrics", "tags", "--metric", "foo", "--json"}, true},
		{[]string{"--metric", "foo", "--json"}, true},
		{[]string{"--", "--json"}, false},
		{[]string{}, false},
		// last-wins: later flag overrides earlier one
		{[]string{"--json", "--json=false"}, false},
		{[]string{"--json=false", "--json"}, true},
		{[]string{"--json", "--json=false", "--json"}, true},
	}
	for _, tc := range cases {
		got := ParseJSONFlag(tc.args)
		if got != tc.want {
			t.Errorf("ParseJSONFlag(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

// TestJSONErrorWhenUnknownFlagPrecedesJSON verifies that error output is JSON
// when --json appears after an unknown flag (cobra fails before parsing --json,
// so ParseJSONFlag pre-scan is required).
func TestJSONErrorWhenUnknownFlagPrecedesJSON(t *testing.T) {
	t.Parallel()
	args := []string{"metrics", "tags", "--metric", "foo", "--json"}

	root := NewRootCommand("dev")
	root.SetArgs(args)
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)

	execErr := root.Execute()
	if execErr == nil {
		t.Fatal("expected error from unknown flag --metric")
	}

	// cobra should NOT have parsed --json (it errored on --metric first)
	cobraParsed, _ := root.PersistentFlags().GetBool("json")

	// pre-scan must detect --json regardless of cobra
	preParsed := ParseJSONFlag(args)
	if !preParsed {
		t.Fatal("ParseJSONFlag must return true when --json is present")
	}

	// using pre-scan result: error must be JSON
	var buf bytes.Buffer
	WriteError(&buf, execErr, preParsed)
	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("error output must be valid JSON (cobraParsed=%v): %v\noutput=%q", cobraParsed, err, buf.String())
	}
	if _, ok := got["error"]; !ok {
		t.Errorf("JSON error object missing 'error' key: %v", got)
	}
}

func TestRootSuppressesUsageOnRunError(t *testing.T) {
	t.Parallel()
	root := NewRootCommand("dev")
	root.AddCommand(&cobra.Command{
		Use: "fail",
		RunE: func(*cobra.Command, []string) error {
			return errors.New("intentional failure")
		},
	})

	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"fail"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error from fail command")
	}
	combined := out.String() + errBuf.String()
	if strings.Contains(combined, "Usage:") {
		t.Errorf("cobra should not print Usage on RunE error, got %q", combined)
	}
}
