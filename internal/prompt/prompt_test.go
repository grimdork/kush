package prompt

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestExpandPromptBuiltins(t *testing.T) {
	ctx := context.Background()
	res := expandPrompt(ctx, "%T %t %H %p %P %%", false)
	if res == "" {
		t.Fatalf("expected non-empty result for builtins, got empty")
	}
}

func TestExpandPromptEscapes(t *testing.T) {
	ctx := context.Background()
	res := expandPrompt(ctx, `\\% \[ \{`, false)
	expected := "\\% [ {"
	if res != expected {
		t.Fatalf("escape handling failed: got %q expected %q", res, expected)
	}
}

func TestExpandPromptExternalDisabled(t *testing.T) {
	ctx := context.Background()
	res := expandPrompt(ctx, "[echo hi]", false)
	if res != "" {
		t.Fatalf("expected empty when external disabled, got %q", res)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// sleep longer than context to force timeout
	res := runCommand(ctx, "sleep 0.1 && echo done")
	if res != "" {
		t.Fatalf("expected empty on timeout, got %q", res)
	}
}

func TestScriptExecution(t *testing.T) {
	// create a temporary script
	f := "/tmp/kush_prompt_test.sh"
	_ = os.WriteFile(f, []byte("#!/bin/sh\necho script-ok"), 0755)
	defer os.Remove(f)
	ctx := context.Background()
	res := expandPrompt(ctx, "{"+f+"}", true)
	if res != "script-ok" {
		t.Fatalf("expected script-ok, got %q", res)
	}
}
