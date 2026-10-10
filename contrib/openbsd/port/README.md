# OpenBSD port for kwd

Port files for submitting **kwd** to the official OpenBSD ports tree.

## Version bump

From the kwd repo root, after updating **`VERSION`**, run **`make port-openbsd-sync`** to refresh **DISTNAME**, **PKGNAME**, **MASTER_SITES**, and **DISTFILES** in this **Makefile**.

## distinfo

**`distinfo`** is not shipped here. After **`make fetch`** (or a local tarball), run **`make makesum`** in your OpenBSD ports checkout and include **`distinfo`** in the diff to **ports@openbsd.org**.

## Layout

Copy this directory to **`/usr/ports/sysutils/kwd/`**:

```bash
cd /usr/ports
mkdir -p sysutils/kwd
cp -r /path/to/kwd/contrib/openbsd/port/* sysutils/kwd/
cd sysutils/kwd
```

## Test with a local tarball

1. **`make port-openbsd-sync`** from the kwd repo (updates this **Makefile**).
2. **`make dist-openbsd`** (optional **`OPENBSD_ARCH=arm64`**). Output: **`dist/kwd_v<version>_openbsd_<arch>.tar.gz`**.
3. Copy into **DISTDIR** or use **`MASTER_SITES=file:///.../`** for **`make fetch`** / **`make install`** (do not commit **`file:`** URLs to the official tree).

See the [OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html).
