package builtins

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grimdork/kush/internal/aliases"
	"github.com/grimdork/kush/internal/log"
	"os/exec"
)

func init() {
	Register("alias", aliasHandler)
	Register("unalias", unaliasHandler)
	Register("reload", reloadHandler)
}

func aliasHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 1 {
		al, _ := aliases.Load()
		if al == nil {
			return true
		}
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
}

func unaliasHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
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
}

func reloadHandler(b *Builtins, line string) bool {
	if _, err := aliases.Reload(); err != nil {
		log.Errorf("reload: failed: %v", err)
	} else {
		log.Debugf("aliases reloaded")
	}

	// Re-read environment variables that affect internal settings.
	if b.pp != nil {
		b.pp.Invalidate()
		log.Debugf("prompt provider invalidated")
	}
	return true
}
