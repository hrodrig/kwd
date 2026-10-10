# Docs

Operator runbooks and in-cluster manifests belong in **[kwd-selfhosted](https://github.com/hrodrig/kwd-selfhosted)** when that repo is published. This tree holds product-facing assets for the CLI README.

## Hero

| Asset | Role |
|-------|------|
| [`kwd-hero-oss.jpg`](kwd-hero-oss.jpg) | README hero banner |

<a id="terminal-demo-vhs"></a>
## Terminal demo (VHS)

| Asset | Role |
|-------|------|
| [`demo.tape`](demo.tape) | Charmbracelet [VHS](https://github.com/charmbracelet/vhs) script |
| [`demo-kwd.yaml`](demo-kwd.yaml) | Minimal config for `analyze` in the tape (no cluster required) |
| [`demo.gif`](demo.gif) | Recorded GIF embedded in the root README |

From the repository root (binary on `PATH`, ldflags from `make build` / `make install`):

```bash
make build
export PATH="$(pwd)/bin:$PATH"
bash -c "vhs docs/demo.tape"
```

Use `bash -c` so the recorder does not inherit zsh/Oh My Zsh prompts.

**Re-record after every `VERSION` bump** — `kwd version` in the GIF must match **`VERSION`**.
