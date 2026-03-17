Phase 1 TODOs

- Persistent history (implemented)
- PTY-backed runner (replace execpt.RunShell)
- Tab completion and zsh-like suggestions
- Opt+left/right word movement and opt+backspace
- Ctrl+H history viewer (tcell overlay)
- Integrate local repos (name, base) as builtins
- Tengo plugin scaffold and mixed-script format
- Builtin networking tools (ping, dig wrappers)
- Checksums: implement md5/sha* in builtins
- Config: internal-first vs PATH-first policy
- Docker/K8s detection helpers
- Tests and CI

Phase 2 TODOs

- Spelling/grammar integration (sajari/fuzzy + LanguageTool)
- Optional macOS native spell bridge (CGO) — postponed
- Plugin registry and loader for Tengo scripts
- More builtins from grimdork repos
