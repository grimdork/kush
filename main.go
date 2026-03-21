package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/grimdork/climate/arg"
	"github.com/grimdork/kush/internal/scripting"
	"github.com/grimdork/kush/internal/shell"
)

// version is populated at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	opt := arg.New("kush", "Kubernetes utility shell.")
	opt.SetDefaultHelp(true)
	if err := opt.SetFlag(arg.GroupDefault, "v", "version", "print version and exit"); err != nil {
		log.Fatal(err)
	}
	// No short flag for install-scripts
	if err := opt.SetFlag(arg.GroupDefault, "", "install-scripts", "install bundled scripts into the user's blessed scripts directory (post-install)"); err != nil {
		log.Fatal(err)
	}

	if err := opt.Parse(os.Args[1:]); err != nil {
		// If there were no args, continue to interactive shell.
		if err != arg.ErrNoArgs {
			log.Fatal(err)
		}
	}

	if opt.GetBool("version") || opt.GetBool("v") {
		fmt.Println(version)
		return
	}

	if opt.GetBool("install-scripts") {
		eng := scripting.New(nil)
		target := eng.BlessedDir()
		if err := installBundledScripts(target); err != nil {
			log.Fatal(err)
		}
		fmt.Println("installed scripts to:", target)
		return
	}

	// If invoked as: kush <script.tengo|.t|<blessed-name>> [args...], run the script and exit.
	if len(opt.Args) > 0 {
		first := opt.Args[0]
		ext := filepath.Ext(first)
		eng := scripting.New(nil)
		// remaining args are passed to the script (program name stripped)
		scriptArgs := []string{}
		if len(opt.Args) > 1 {
			scriptArgs = opt.Args[1:]
		}
		if ext == ".t" || ext == ".tengo" {
			if err := eng.RunFile(first, scriptArgs); err != nil {
				log.Fatal(err)
			}
			return
		}
		// If no extension, try running a blessed script by that name
		if ext == "" {
			err := eng.RunBlessed(first, scriptArgs)
			if err == nil {
				return
			}

			log.Fatal(err)
		}
	}

	if err := shell.Run(); err != nil {
		log.Fatal(err)
	}
}

func installBundledScripts(targetDir string) error {
	// find source examples directory. Try executable dir first, then cwd.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "examples"),
		"examples",
	}
	var src string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			src = c
			break
		}
	}
	if src == "" {
		return fmt.Errorf("bundled scripts not found (looked in %v)", candidates)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	// shebang path: prefer the actual executable path
	shebang := exe

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read bundled scripts: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !(strings.HasSuffix(name, ".tengo") || strings.HasSuffix(name, ".t")) {
			continue
		}
		inPath := filepath.Join(src, name)
		outPath := filepath.Join(targetDir, name)
		b, err := os.ReadFile(inPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", inPath, err)
		}
		text := string(b)
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
			// replace shebang
			if len(lines) == 1 {
				text = "#!" + shebang + "\n"
			} else {
				text = "#!" + shebang + "\n" + lines[1]
			}
		} else {
			// prepend shebang
			if len(text) > 0 && !strings.HasSuffix(text, "\n") {
				text = "#!" + shebang + "\n" + text + "\n"
			} else {
				text = "#!" + shebang + "\n" + text
			}
		}
		if err := os.WriteFile(outPath, []byte(text), 0o755); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Println("installed", name)
	}
	return nil
}
