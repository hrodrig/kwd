# OpenBSD port for kwd (Kubernetes readiness watchdog CLI)
# Copy to /usr/ports/sysutils/kwd/ and run: make makesum
# Submit: send diff to ports@openbsd.org

COMMENT =	Kubernetes workload readiness watchdog (CLI)
DISTNAME =	kwd_v0.2.0_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}
PKGNAME =	kwd-0.2.0
CATEGORIES =	sysutils
HOMEPAGE =	https://github.com/hrodrig/kwd
MAINTAINER =	hrodrig@usb.ve

# MIT
PERMIT_PACKAGE =	Yes
PERMIT_DISTFILES =	Yes

# Local test before release: copy tarball to DISTDIR or use
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
# See README.md "Test with a local tarball".
MASTER_SITES =	https://github.com/hrodrig/kwd/releases/download/v0.2.0/
DISTFILES =	kwd_v0.2.0_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX =	.tar.gz

ONLY_FOR_ARCHS =	amd64 arm64 aarch64
NO_BUILD =	Yes
NO_TEST =	Yes
PKG_ARCH =	*

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/kwd ${PREFIX}/bin/kwd
	${INSTALL_PROGRAM} ${WRKDIR}/kubectl-kwd ${PREFIX}/bin/kubectl-kwd
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/kwd.1 ${PREFIX}/man/man1/kwd.1
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/kubectl-kwd.1 ${PREFIX}/man/man1/kubectl-kwd.1
	${INSTALL_DATA_DIR} ${PREFIX}/share/doc/kwd
	${INSTALL_DATA} ${WRKDIR}/share/doc/kwd/LICENSE ${PREFIX}/share/doc/kwd/
	${INSTALL_DATA_DIR} ${PREFIX}/share/examples/kwd
	${INSTALL_DATA} ${WRKDIR}/share/examples/kwd/kwd.sample.yml ${PREFIX}/share/examples/kwd/

.include <bsd.port.mk>
