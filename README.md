# kush — Kubernetes utility shell

Kush is a terminal shell with built-in networking utilities, written in Go. It provides a custom
single-line editor (no readline or go-prompt dependency), tab completion, command aliases, persistent
history, and a PTY-backed command runner so interactive programs behave correctly.

## Features

- **Line editor** — single-line editing with cursor movement, history
  navigation, and word-level operations. No external TUI libraries.
- **Tab completion** — context-aware completion for commands (from `$PATH`)
  and file paths, with a two-row candidate display and Tab/Shift+Tab cycling.
- **PTY runner** — commands run inside a pseudoterminal (`openpty` on macOS,
  `posix_openpt` on Linux) with automatic fallback to plain `exec` on
  unsupported platforms.
- **Aliases** — loaded from `~/.kush_aliases` (or `$KUSH_ALIASES`), with
  two-pass chained expansion and live reload via `reload` or SIGHUP.
- **Pipelines and redirects** — builtins can pipe into external commands
  and redirect to files (`>`, `>>`).
- **Builtins** — `cd`, `export`, `history`, `alias`, `unalias`, `reload`,
  `which`, `help`, `checksum`, plus HTTP (`get`, `post`, `put`, `delete`,
  `head`, `fetch`) and scripting (`run`, `eval`).
- **Config** — optional `~/.kush_config` for key=value settings like
  `PATH_FIRST`.

## Building

```sh
make          # produces ./kush
make clean    # removes binaries
```

Requires Go 1.25+ and cgo on macOS (for `openpty`).

## Key bindings

| Key                   | Action                       |
|-----------------------|------------------------------|
| Tab                   | Complete / cycle forward      |
| Shift+Tab             | Cycle backward                |
| Left / Right          | Move cursor                  |
| Up / Down             | Navigate history             |
| Home / End            | Jump to start / end of line  |
| Alt+Left / Alt+Right  | Move by word                 |
| Backspace             | Delete character left        |
| Alt+Backspace         | Delete word left             |
| Delete                | Delete word right            |
| Ctrl+W                | Kill word left               |
| Ctrl+U                | Kill to start of line        |
| Ctrl+K                | Kill to end of line          |
| Ctrl+C                | Clear current line           |
| Ctrl+H                | Open history viewer          |
| Ctrl+D                | Exit (EOF)                   |

## Terminal configuration

Kush expects Option/Alt to send an Escape prefix (Meta mode). If word-delete
shortcuts don't work, configure your terminal:

**iTerm2:** Preferences → Profiles → Keys → set Left Option to `Esc+`.

**Terminal.app:** Preferences → Profiles → Keyboard → add a mapping for
Option+Backspace that sends `\033\177`.

**Alacritty:** set `alt_send_escape: true` or add a key mapping for
Option+Backspace → `\x1b\x7f`.

To inspect what your terminal sends for a key, run `cat -v` and press it.

## Debug mode

Set `KUSH_DEBUG` to control diagnostic output on stderr:

- `0` (default) — quiet; only real errors are printed.
- `1` — verbose; alias loading, reload events, and runner diagnostics.
- `2` — trace; detailed PTY lifecycle, termios state, and goroutine events.

Set `KUSH_KEYDEBUG` for key input diagnostics:

- `1` — log raw key codes to stderr.
- `2` — key codes plus cursor position debug.
- `3` — all of the above.

## Aliases

Aliases are loaded from `~/.kush_aliases` (override with `$KUSH_ALIASES`).
Supported formats:

```
alias ll='ls -la'
ll='ls -la'
ll=ls -la
```

Chained aliases work: if `la` expands to `ls -la` and `ls` expands to
`ls --color=yes`, the result is `ls --color=yes -la` (without duplicate flags).

Reload aliases from within the shell with `reload`, or from outside with
`kill -HUP <pid>`.

To bypass alias expansion for a single command, wrap it in parentheses:

```
(ls)          # runs /bin/ls directly, even if ls is aliased
(grep) foo    # skips any grep alias
```

## Pipelines and redirects

Builtins can participate in pipelines and redirects, just like external
commands:

```
history | grep ssh           # pipe builtin output to an external command
get https://example.com | grep title
history > ~/cmds.txt         # redirect to file
history >> ~/cmds.txt        # append to file
```

The first command in a pipeline can be a builtin; its output is captured and
fed into the rest of the pipeline, which is executed via `sh -c`. Redirect
operators (`>`, `>>`) work the same way.

Quotes inside commands are respected — pipes and redirects inside quoted
strings are treated as literal characters.

## History viewer

Press **Ctrl+H** to open a full-screen history viewer with search and
navigation:

| Key            | Action                              |
|----------------|-------------------------------------|
| ↑ / ↓ / j / k | Move selection                      |
| PgUp / PgDn    | Scroll by page                      |
| g / G          | Jump to top / bottom                |
| Home / End     | Jump to top / bottom                |
| /              | Start incremental search            |
| Esc (in search)| Return to list, keeping filter      |
| Enter          | Select entry into the command line  |
| d / Delete     | Mark entry for deletion             |
| Esc / q        | Close viewer                        |

Deleted entries are removed from both memory and `~/.kush_history` when the
viewer closes.

## Project structure

```
main.go                          Entry point
internal/
  shell/
    shell.go                     REPL loop
    pipeline.go                  Pipeline and redirect parsing/execution
    execute.go                   Pipeline executor
  ed/                            Line editor and termios helpers
    lineeditor.go                Editor implementation
    history_viewer.go            Ctrl+H history browser
    term_darwin.go               macOS termios (raw mode)
    term_linux.go                Linux termios
  completion/completion.go       Tab completion (commands + paths)
  runner/                        Command execution
    pty_runner.go                PTY runner + plain-exec fallback
    run_shell.go                 Shell-mode execution (darwin)
    run_shell_linux.go           Shell-mode execution (linux)
    pty_darwin.go                openpty via cgo (macOS)
    pty_linux.go                 posix_openpt (Linux)
    pty_unsupported.go           Stub for other platforms
    winsize.go                   SIGWINCH propagation
  aliases/aliases.go             Alias loading, expansion, persistence
  builtins/
    builtins.go                  Builtin registry and dispatch
    register.go                  Init-based handler registration
    cd.go, export.go, history.go, alias.go, which.go, checksum.go
    help.go, help_print.go       Help text (cfmt coloured)
    http.go                      HTTP builtins (get, post, put, etc.)
    script.go                    Tengo scripting (run, eval)
  httpclient/client.go           Shared HTTP client
  scripting/
    engine.go                    Tengo runtime
    http.go                      HTTP module for Tengo scripts
  config/config.go               Config file loader
  log/log.go                     Levelled logging
```


KUSH prompt configuration

Kush uses KUSH_PROMPT as the canonical prompt configuration. If KUSH_PROMPT is
unset, the provider falls back to PROMPT / PROMPT_CMD for compatibility.

KUSH_PROMPT supports a minimal token language:
- %% → literal %
- %T → full datetime (YYYY-MM-DD HH:MM:SS)
- %t → time (HH:MM:SS)
- %H → hostname
- %h → hour (HH)
- %m → minute (MM)
- %s → second (SS)
- %p → last path component (basename of PWD)
- %P → full path (PWD)

Dynamic tokens:
- {path/to/script} → run a local script or executable and substitute its stdout (trimmed). The script is executed via sh -c for now; planned: embedded Tengo runner.
- [command] → run external command via sh -c and substitute stdout. For safety, external commands are disabled by default. Enable with KUSH_PROMPT_ALLOW_EXTERNAL=1.

Escaping:
- Use backslash to escape special characters: \% \[ \{ \\

Timeouts and caching:
- PROMPT_TIMEOUT_MS controls per-prompt timeout (milliseconds).
- PROMPT_TTL controls caching (e.g., avoid re-running expensive tokens every prompt).

Security:
- External commands can execute arbitrary code; keep KUSH_PROMPT_ALLOW_EXTERNAL disabled unless you trust the prompt string source.
- Internal script support via Tengo is planned for safer, embeddable prompt scripts.

Examples:
- KUSH_PROMPT="%p %t $ "
- KUSH_PROMPT="{~/.kush/scripts/git_branch} %t $ "
- Enable external commands: export KUSH_PROMPT_ALLOW_EXTERNAL=1


---

