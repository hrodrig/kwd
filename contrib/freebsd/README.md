# FreeBSD port for kwd

Port files for building and installing **kwd** (CLI + `kubectl-kwd` shim; no rc.d).

## Install from port

When the port is in the official tree:

```bash
cd /usr/ports/sysutils/kwd
make install
```

Local port (copy `Makefile`, `pkg-plist`, `pkg-descr` from this directory):

```bash
cd ~/ports/sysutils/kwd
make install
```

After changing port files: `make deinstall && make clean && make install`.

## Test with a local distfile

1. From the **kwd** repo root, sync **PORTVERSION** with **`VERSION`**:

   ```bash
   make port-freebsd-sync
   ```

2. Build the tarball expected by **DISTFILES** (default arch **amd64**; override with **`FREEBSD_ARCH=arm64`**):

   ```bash
   make dist-freebsd
   ```

   Output: `dist/kwd_v<version>_freebsd_<arch>.tar.gz`.

3. Copy into **DISTDIR** or use **`MASTER_SITES=file:///.../`** as in the [FreeBSD Porter's Handbook](https://docs.freebsd.org/en/books/porters-handbook/).

The tarball contains: `kwd`, `kubectl-kwd`, `share/man/man1/kwd.1`, `share/man/man1/kubectl-kwd.1`, `share/doc/kwd/LICENSE`, `share/examples/kwd/kwd.sample.yml`.
