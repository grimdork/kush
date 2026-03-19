package completion

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Complete returns the replacement start index and candidate slice for the
// given line buffer and cursor position. It is safe to call from the editor.
// If no completer is configured, returns start=pos and no candidates.

var (
	builtinProvider func() []string
	aliasProvider   func() []string
)

// SetProviders registers simple provider functions for builtins and aliases.
func SetProviders(builtinFn func() []string, aliasFn func() []string) {
	builtinProvider = builtinFn
	aliasProvider = aliasFn
}

// Complete performs a simple word-based completion.
func Complete(line string, pos int) (int, []string) {
	if pos > len(line) {
		pos = len(line)
	}
	// find start of current token
	start := pos
	for start > 0 && line[start-1] != ' ' {
		start--
	}
	prefix := line[start:pos]
	// split first token to detect command context
	fields := strings.Fields(line[:start])
	var cmd string
	if len(fields) > 0 {
		cmd = fields[0]
	}
	candidates := []string{}
	// If the prefix looks like a path, do file completion regardless of position.
	isPath := strings.HasPrefix(prefix, "~") || strings.HasPrefix(prefix, "/") ||
		strings.HasPrefix(prefix, "./") || strings.HasPrefix(prefix, "../")

	// If at first token (no previous fields), suggest builtins, aliases, PATH
	if start == 0 && !isPath {
		if builtinProvider != nil {
			for _, b := range builtinProvider() {
				if strings.HasPrefix(b, prefix) {
					candidates = append(candidates, b)
				}
			}
		}
		if aliasProvider != nil {
			for _, a := range aliasProvider() {
				if strings.HasPrefix(a, prefix) {
					candidates = append(candidates, a)
				}
			}
		}
		// PATH
		pathenv := os.Getenv("PATH")
		for _, dir := range strings.Split(pathenv, string(os.PathListSeparator)) {
			if dir == "" {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, prefix) {
					// check executable
					fi, err := e.Info()
					if err == nil {
						mode := fi.Mode()
						if !mode.IsDir() && mode&0111 != 0 {
							candidates = append(candidates, name)
						}
					}
				}
			}
		}
	} else {
		// Not first token: file completion, with special-casing for `cd` and `which` and `help`
		if cmd == "cd" {
			candidates = completeFilesystem(prefix, true)
			return start, candidates
		}
		if cmd == "which" {
			// behave like first-token PATH completion
			pathenv := os.Getenv("PATH")
			for _, dir := range strings.Split(pathenv, string(os.PathListSeparator)) {
				if dir == "" {
					continue
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					name := e.Name()
					if strings.HasPrefix(name, prefix) {
						fi, err := e.Info()
						if err == nil {
							mode := fi.Mode()
							if !mode.IsDir() && mode&0111 != 0 {
								candidates = append(candidates, name)
							}
						}
					}
				}
			}
			return start, candidates
		}
		if cmd == "help" {
			// suggest builtins and aliases
			if builtinProvider != nil {
				for _, b := range builtinProvider() {
					if strings.HasPrefix(b, prefix) {
						candidates = append(candidates, b)
					}
				}
			}
			if aliasProvider != nil {
				for _, a := range aliasProvider() {
					if strings.HasPrefix(a, prefix) {
						candidates = append(candidates, a)
					}
				}
			}
			return start, candidates
		}
		// default: filesystem entries
		candidates = completeFilesystem(prefix, false)
	}

	// dedupe and sort
	m := map[string]struct{}{}
	for _, c := range candidates {
		m[c] = struct{}{}
	}
	list := make([]string, 0, len(m))
	for k := range m {
		list = append(list, k)
	}
	sort.Strings(list)
	return start, list
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		h := os.Getenv("HOME")
		rest := strings.TrimPrefix(p, "~")
		if rest == "" || rest == "/" {
			return h + "/"
		}
		return h + rest
	}
	return p
}

// completeFilesystem handles path-aware file/directory completion.
// It preserves the user's original prefix form (~, ./, ../, /) in candidates.
func completeFilesystem(prefix string, dirsOnly bool) []string {
	expanded := expandPath(prefix)

	var dir, base string
	if strings.HasSuffix(expanded, "/") {
		// User typed a complete directory — list its contents.
		dir = expanded
		base = ""
	} else {
		dir = filepath.Dir(expanded)
		base = filepath.Base(expanded)
	}
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Figure out the prefix to prepend so candidates match the user's
	// original typing (~/foo, ./bar, /etc).
	var userDir string
	if strings.HasSuffix(prefix, "/") {
		userDir = prefix
	} else if strings.Contains(prefix, "/") {
		userDir = prefix[:strings.LastIndex(prefix, "/")+1]
	} else {
		userDir = ""
	}

	var candidates []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if dirsOnly && !e.IsDir() {
			continue
		}
		if e.IsDir() {
			candidates = append(candidates, userDir+name+"/")
		} else {
			candidates = append(candidates, userDir+name)
		}
	}
	return candidates
}
