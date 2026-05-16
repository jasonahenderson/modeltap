# 2026-05-11 Session: PATCH-0030 terminal selection default

## Summary

Adjusted PATCH-0030 so terminal-native selection is the default in the
production shell and shell demo. Removed `tea.WithMouseAllMotion()` from
startup, initialized the shell state in selection mode, and kept `/select`
as a runtime toggle into optional chat mouse-scroll mode and back.

Updated shell docs, embedding examples, PATCH-0030, the patch index, and the
v0.3.0 retrospective to describe the new default.

## Verification

- `go test ./internal/harnessshell ./internal/cli`
- `go test ./...`

Manual click-drag/copy smoke verification in a real terminal is still pending.
