# Docs

Operator runbooks and in-cluster manifests belong in **[kwd-selfhosted](https://github.com/hrodrig/kwd-selfhosted)** when that repo is published. This tree holds product-facing assets for the CLI README.

## Hero

| Asset | Role |
|-------|------|
| [`kwd-hero-oss.jpg`](kwd-hero-oss.jpg) | README hero banner |

## Terminal demo (VHS) — planned

Family CLIs ([kzero](https://github.com/hrodrig/kzero), [groot](https://github.com/hrodrig/groot), [pgwd](https://github.com/hrodrig/pgwd)) ship a Charmbracelet [VHS](https://github.com/charmbracelet/vhs) tape → `demo.gif`.

**Not in-tree yet:** `docs/demo.tape` / `docs/demo.gif`. Do not link a missing GIF from the root README. When added:

```bash
make install
PATH="$(go env GOPATH)/bin:$PATH" vhs docs/demo.tape
```

`kwd version` in the tape must match **`VERSION`**.
