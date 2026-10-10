# OpenBSD — kwd

**kwd** is a **CLI** only: there is no bundled **rc.d** script in this repository. Operators run **`kwd`** from cron, CI, or their own wrappers (see **[hrodrig/kwd-selfhosted]** for operator assets).

Official port skeleton: **`contrib/openbsd/port/`** (submit to **ports@openbsd.org**).

Release tarballs match **`DISTFILES`** in that port (binaries **`kwd`** and **`kubectl-kwd`**, man pages, **`share/doc/kwd/LICENSE`**, and **`share/examples/kwd/kwd.sample.yml`**). Build a matching tarball locally with **`make dist-openbsd`** from the repo root.
