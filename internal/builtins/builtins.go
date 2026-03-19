package builtins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grimdork/kush/internal/aliases"
	"github.com/grimdork/kush/internal/log"
	"github.com/grimdork/kush/internal/prompt"
)

// Builtins provides handling for built-in commands that are executed directly by the shell rather than via exec.
type Builtins struct{
	pp *prompt.Provider
}

// New returns a new Builtins instance. Pass a prompt.Provider so builtins can invalidate the prompt cache on env changes.
func New(pp *prompt.Provider) *Builtins { return &Builtins{pp: pp} }

// Handle returns true if the line was handled by a builtin.
func (b *Builtins) Handle(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "help":
		printHelp(parts)
		return true
	case "cd":
		dir := "~"
		if len(parts) > 1 {
			dir = parts[1]
		}
		if dir == "~" {
			dir = os.Getenv("HOME")
		}
		if err := os.Chdir(dir); err != nil {
			log.Errorf("cd: %v", err)
		}
		return true
	case "history":
		home := os.Getenv("HOME")
		f := filepath.Join(home, ".kush_history")
		data, _ := os.ReadFile(f)
		fmt.Print(string(data))
		return true
	case "checksum":
		if len(parts) < 3 {
			log.Errorf("usage: checksum [md5|sha1|sha256] file")
			return true
		}
		algo := parts[1]
		file := parts[2]
		switch algo {
		case "md5", "sha1", "sha256":
			// placeholder — implement later
			fmt.Printf("checksum %s %s\n", algo, file)
		default:
			log.Errorf("unknown algorithm")
		}
		return true
	case "export":
		// export KEY=VALUE or export KEY VALUE
		rest := strings.TrimSpace(strings.TrimPrefix(line, "export"))
		if rest == "" {
			// list environment
			for _, e := range os.Environ() {
				fmt.Println(e)
			}
			return true
		}
		var key, val string
		if strings.Contains(rest, "=") {
			parts2 := strings.SplitN(rest, "=", 2)
			key = strings.TrimSpace(parts2[0])
			val = parts2[1]
		} else {
			parts2 := strings.Fields(rest)
			if len(parts2) == 1 {
				// export KEY -> export existing value
				key = parts2[0]
				val = os.Getenv(key)
			} else if len(parts2) >= 2 {
				key = parts2[0]
				val = strings.Join(parts2[1:], " ")
			}
		}
		if key != "" {
			os.Setenv(key, val)
			// If we have a prompt provider, invalidate its cache so the running REPL picks up changes immediately.
			if b.pp != nil {
				b.pp.Invalidate()
			}
		}
		return true
	case "alias":
		// alias -> list or reload (-r)
		if len(parts) == 1 {
			al, _ := aliases.Load()
			if al == nil {
				return true
			}
			// list aliases in sorted order for consistent output
			list := al.All()
			keys := make([]string, 0, len(list))
			for k := range list {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("alias %s='%s'\n", k, list[k])
			}
			return true
		}
		// alias -r -> reload from disk
		if len(parts) == 2 && parts[1] == "-r" {
			if _, err := aliases.Reload(); err != nil {
				log.Errorf("alias: reload failed: %v", err)
			} else {
				if log.Level() >= 1 {
					log.Debugf("aliases reloaded")
				}
			}
			return true
		}
		// alias name='value' or alias name=value
		rest := strings.TrimSpace(strings.TrimPrefix(line, "alias"))
		if rest == "" {
			return true
		}
		parts2 := strings.SplitN(strings.TrimSpace(rest), "=", 2)
		if len(parts2) != 2 {
			return true
		}
		name := strings.TrimSpace(parts2[0])
		val := strings.TrimSpace(parts2[1])
		if len(val) >= 2 {
			if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
				val = val[1 : len(val)-1]
			}
		}
		al, _ := aliases.Load()
		if al == nil {
			al = &aliases.Aliases{}
		}
		al.Set(name, val)
		if err := al.Save(); err != nil {
			log.Errorf("alias: save failed: %v", err)
		} else {
			// warn if the expansion's command does not exist in PATH (debug level >=1)
			toks := strings.Fields(val)
			if len(toks) > 0 {
				if p, _ := exec.LookPath(toks[0]); p == "" {
					if log.Level() >= 1 {
						log.Warnf("alias: warning: target %q not found in PATH", toks[0])
					}
				}
			}
		}
		return true
	case "unalias":
		if len(parts) < 2 {
			log.Errorf("usage: unalias name")
			return true
		}
		al, _ := aliases.Load()
		if al == nil {
			return true
		}
		al.Unset(parts[1])
		if err := al.Save(); err != nil {
			log.Errorf("unalias: save failed: %v", err)
		}
		return true
	case "reload":
		// reload aliases from disk into cache
		if _, err := aliases.Reload(); err != nil {
			log.Errorf("reload: failed: %v", err)
		} else {
			log.Debugf("aliases reloaded")
		}
		return true
	case "which":
		// simple which: print the path for each argument
		if len(parts) == 1 {
			log.Errorf("usage: which prog [prog...]")
			return true
		}
		for _, a := range parts[1:] {
			if p, _ := exec.LookPath(a); p != "" {
				fmt.Println(p)
			} else {
				log.Errorf("%s: not found in PATH", a)
			}
		}
		return true
	}
	return false
}
