package aliases

import (
	"strings"
	"testing"
)

func TestChainedAliasExpansion(t *testing.T) {
	a := &Aliases{m: map[string]string{
		"la": "ls -la",
		"ls": "ls --color=yes",
	}}

	in := "la"
	out := a.Expand(in)
	// simulate shell behavior: single-pass then conditional second-pass
	first := out
	if first != in {
		origTok := ""
		newTok := ""
		if f := split(first); len(f) > 0 {
			newTok = f[0]
		}
		if s := split(in); len(s) > 0 {
			origTok = s[0]
		}
		if origTok != newTok {
			out = a.Expand(first)
		}
	}

	// ensure --color appears exactly once
	if count := countSub(out, "--color"); count != 1 {
		t.Fatalf("unexpected duplicate --color in %q", out)
	}
}

func split(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		out = append(out, f)
	}
	return out
}

func countSub(s, sub string) int {
	c := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			c++
		}
	}
	return c
}
