package prompt_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grimdork/kush/internal/prompt"
)

func TestExpandPromptBuiltins(t *testing.T) {
	os.Unsetenv("KUSH_PROMPT")
	p := &prompt.Provider{Static: "%T %t %H %p %P %%"}
	res := p.Get()
	if res == "" {
		t.Fatalf("expected non-empty result for builtins, got empty")
	}
}

func TestExpandPromptEscapes(t *testing.T) {
	os.Unsetenv("KUSH_PROMPT")
	p := &prompt.Provider{Static: `\\% \[ \{`}
	res := p.Get()
	expected := "\\% [ {"
	if res != expected {
		t.Fatalf("escape handling failed: got %q expected %q", res, expected)
	}
}

func TestExpandPromptExternalDisabled(t *testing.T) {
	os.Unsetenv("KUSH_PROMPT")
	p := &prompt.Provider{Static: "[echo hi]", AllowExternal: false}
	res := p.Get()
	// When external commands are disabled, Provider falls back to Static if
	// expansion yields an empty string. Ensure the returned value equals the
	// original Static string and was not replaced by command output.
	if res != p.Static {
		t.Fatalf("expected provider to return static when external disabled, got %q", res)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	os.Unsetenv("KUSH_PROMPT")
	p := &prompt.Provider{Cmd: "[sleep 0.1 && echo done]", AllowExternal: true, Timeout: 10 * time.Millisecond}
	res := p.Get()
	if strings.Contains(res, "done") {
		t.Fatalf("expected timeout to prevent command output, got %q", res)
	}
}

func TestScriptExecution(t *testing.T) {
	os.Unsetenv("KUSH_PROMPT")
	// create a temporary script
	f := "/tmp/kush_prompt_test.sh"
	_ = os.WriteFile(f, []byte("#!/bin/sh\necho script-ok"), 0755)
	defer os.Remove(f)
	p := &prompt.Provider{Cmd: "{" + f + "}", AllowExternal: true}
	res := p.Get()
	if res != "script-ok" {
		t.Fatalf("expected script-ok, got %q", res)
	}
}
