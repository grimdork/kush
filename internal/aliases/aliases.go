package aliases

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/grimdork/kush/internal/log"
)

// Aliases holds a simple map of name->expansion.
type Aliases struct {
	m map[string]string
}

// package-level cache so callers get the same parsed aliases without
// re-reading different files at different times (avoids races between the
// builtin reader and shell startup).
var cached *Aliases

// Reload clears the in-memory cache and forces the next Load() to re-read the file.
func Reload() (*Aliases, error) {
	cached = nil
	return Load()
}

// Load loads aliases from $HOME/.kush_aliases.
// Format supported:
//
//	alias ll='ls -la'
//	ll='ls -la'
//	ll=ls -la
func Load() (*Aliases, error) {
	if cached != nil {
		return cached, nil
	}
	home := os.Getenv("HOME")
	// allow overriding alias file via env var for easier testing
	if p := os.Getenv("KUSH_ALIASES"); p != "" {
		paths := []string{p}
		m := make(map[string]string)
		for _, p := range paths {
			f, err := os.Open(p)
			if err != nil {
				log.Warnf("aliases: failed to open %s: %v", p, err)
				continue
			}
			s := bufio.NewScanner(f)
			for s.Scan() {
				ln := strings.TrimSpace(s.Text())
				if ln == "" || strings.HasPrefix(ln, "#") {
					continue
				}
				if strings.HasPrefix(ln, "alias ") {
					ln = strings.TrimSpace(strings.TrimPrefix(ln, "alias "))
				}
				parts := strings.SplitN(ln, "=", 2)
				if len(parts) != 2 {
					continue
				}
				name := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if len(val) >= 2 {
					if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
						val = val[1 : len(val)-1]
					}
				}
				if name != "" && val != "" {
					m[name] = val
				}
			}
			f.Close()
			if log.Level() >= 1 {
				log.Debugf("aliases: loaded %d from %s", len(m), p)
			}
		}
		cached = &Aliases{m: m}
		return cached, nil
	}

	paths := []string{filepath.Join(home, ".kush_aliases")}
	m := make(map[string]string)
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			// don't spam errors for missing optional files
			continue
		}
		s := bufio.NewScanner(f)
		for s.Scan() {
			ln := strings.TrimSpace(s.Text())
			if ln == "" || strings.HasPrefix(ln, "#") {
				continue
			}
			// support `alias name='value'` or `name='value'` or `name=value`
			if strings.HasPrefix(ln, "alias ") {
				ln = strings.TrimSpace(strings.TrimPrefix(ln, "alias "))
			}
			parts := strings.SplitN(ln, "=", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// strip surrounding single or double quotes
			if len(val) >= 2 {
				if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
					val = val[1 : len(val)-1]
				}
			}
			if name != "" && val != "" {
				m[name] = val
			}
		}
		f.Close()
		if len(m) > 0 {
			if log.Level() >= 1 {
				log.Debugf("aliases: loaded %d from %s", len(m), p)
			}
		}
	}
	cached = &Aliases{m: m}
	return cached, nil
}

// Save writes aliases to either $KUSH_ALIASES (if set) or $HOME/.kush_aliases.
func (a *Aliases) Save() error {
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	path := os.Getenv("KUSH_ALIASES")
	if path == "" {
		path = filepath.Join(home, ".kush_aliases")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for k, v := range a.m {
		// write as alias name='value'
		line := "alias " + k + "='" + strings.ReplaceAll(v, "'", "'\"'\"") + "'\n"
		w.WriteString(line)
	}
	w.Flush()
	cached = a
	if log.Level() >= 2 {
		log.Debugf("aliases: saved %d to %s", a.Count(), path)
	}
	return nil
}

// Expand replaces a leading alias if present. Only the first token is checked.
func (a *Aliases) Expand(line string) string {
	if a == nil || strings.TrimSpace(line) == "" {
		return line
	}
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return line
	}
	if v, ok := a.m[tokens[0]]; ok {
		// replace first token with expansion
		remainder := ""
		if len(tokens) > 1 {
			remainder = " " + strings.Join(tokens[1:], " ")
		}
		return v + remainder
	}
	return line
}

// Count returns the number of loaded aliases.
func (a *Aliases) Count() int {
	if a == nil {
		return 0
	}
	return len(a.m)
}

// All returns a copy of the internal map for listing.
func (a *Aliases) All() map[string]string {
	out := make(map[string]string)
	if a == nil {
		return out
	}
	for k, v := range a.m {
		out[k] = v
	}
	return out
}

// Set adds or replaces an alias in memory.
func (a *Aliases) Set(name, val string) {
	if a == nil {
		return
	}
	if a.m == nil {
		a.m = make(map[string]string)
	}
	a.m[name] = val
}

// Unset removes an alias.
func (a *Aliases) Unset(name string) {
	if a == nil || a.m == nil {
		return
	}
	delete(a.m, name)
}
