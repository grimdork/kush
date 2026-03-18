# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]
- Work in progress; tests and CI to be added.

## v0.1.0 - 2026-03-18
- Initial release (v0.1.0):
  - Rewrite renderCandidates to draw candidates downward-only, avoiding DECSTBM/ESC7 upward moves.
  - Fix initial-Tab behaviour to insert the first completion candidate into the buffer.
  - Fix cursor off-by-one (CSI nG is 1-based) so cursor displays at the correct column.
  - Add tab completion docs and debugging notes to README.
  - Add goreleaser config and GitHub Actions workflow for releases (tag-triggered).
