# kwd terminal demo — from repository root, with kwd on PATH:
#   make build && export PATH="$(pwd)/bin:$PATH" && bash -c "vhs docs/demo.tape"
# Or: make install && bash -c "vhs docs/demo.tape"
# Use bash -c (not bare `vhs` from zsh) so the recorder does not inherit zsh/Oh My Zsh prompts.
# Re-record after EVERY VERSION bump — `kwd version` must show the release (e.g. v0.1.0).

Output docs/demo.gif

Require kwd

Set Shell "bash"
Set FontSize 14
Set Width 920
Set Height 560
Set Padding 20

Enter
Sleep 300ms

# Help
Type "kwd --help"
Enter
Sleep 2s

# Version (ldflags when built via make build / make install) — MUST match VERSION
Type "kwd version"
Enter
Sleep 1s

# Sample config (offline)
Type "kwd --print-sample-config | head -n 24"
Enter
Sleep 2s

# Analyze minimal demo profile (no cluster mutations)
Type "kwd analyze --config docs/demo-kwd.yaml"
Enter
Sleep 2s

# Check flags (interval / listen / confirm_*)
Type "kwd check --help"
Enter
Sleep 2s

# Clean exit
Type "exit"
Enter
Sleep 500ms
