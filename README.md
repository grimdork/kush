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
  two-pass chained expansion and live reload via `alias -r` or SIGHUP.
- **Builtins** — `cd`, `history`, `alias`, `unalias`, `reload`, `which`,
  `help`, `checksum` (stub).
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

Reload from within the shell with `alias -r` or from outside with
`kill -HUP <pid>`.

## Project structure

```
main.go                          Entry point
internal/
  shell/shell.go                 REPL loop
  ed/                            Line editor and termios helpers
    lineeditor.go                Editor implementation
    term_darwin.go               macOS termios (raw mode)
    term_linux.go                Linux termios
  completion/completion.go       Tab completion (commands + paths)
  runner/                        Command execution
    pty_runner.go                PTY runner + plain-exec fallback
    run_shell.go                 Shell-mode execution
    pty_darwin.go                openpty via cgo (macOS)
    pty_linux.go                 posix_openpt (Linux)
    pty_unsupported.go           Stub for other platforms
  aliases/aliases.go             Alias loading, expansion, persistence
  builtins/
    builtins.go                  Shell builtins
    help.go                      Help text
  config/config.go               Config file loader
  log/log.go                     Levelled logging
```
