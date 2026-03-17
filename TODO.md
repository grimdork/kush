# kush — TODO

## Phase 1

- [x] Persistent history (~/.kush_history)
- [x] PTY-backed runner (openpty on darwin/linux, plain exec fallback)
- [x] Aliases (~/.kush_aliases, KUSH_ALIASES env override, SIGHUP reload)
- [x] Ctrl+W/U/K kill shortcuts
- [x] Alt/Option word movement and word-delete
- [x] Config loader (~/.kush_config with PATH_FIRST)
- [ ] Tab completion and zsh-like suggestions
- [ ] SIGWINCH propagation to PTY
- [ ] Ctrl+H history viewer (tcell overlay)
- [ ] Integrate local repos (name, base) as builtins
- [ ] Tengo plugin scaffold and mixed-script format
- [ ] Builtin networking tools (ping, dig wrappers)
- [ ] Checksums: implement md5/sha* in builtins
- [ ] Docker/K8s detection helpers
- [ ] Tests and CI

## Phase 2

- [ ] Spelling/grammar integration (sajari/fuzzy + LanguageTool)
- [ ] Optional macOS native spell bridge (CGO)
- [ ] Plugin registry and loader for Tengo scripts
- [ ] More builtins from grimdork repos
