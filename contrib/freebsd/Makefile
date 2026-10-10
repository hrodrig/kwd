PORTNAME=	kwd
PORTVERSION=	0.2.1
DISTVERSIONPREFIX=	v
CATEGORIES=	sysutils
# Local test before release: copy the tarball into DISTDIR (see `make -V DISTDIR`
# from this port directory) using the same name as DISTFILES, or run e.g.
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
# See README.md "Test with a local distfile".
MASTER_SITES=	https://github.com/hrodrig/kwd/releases/download/v${PORTVERSION}/
DISTFILES=	kwd_v${PORTVERSION}_freebsd_${ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX=	.tar.gz

MAINTAINER=	hrodrig@usb.ve
COMMENT=	Kubernetes workload readiness watchdog (CLI)
WWW=		https://github.com/hrodrig/kwd
LICENSE=	MIT

NO_BUILD=	yes
NO_ARCH=	yes

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/kwd ${STAGEDIR}${PREFIX}/bin/kwd
	${INSTALL_PROGRAM} ${WRKDIR}/kubectl-kwd ${STAGEDIR}${PREFIX}/bin/kubectl-kwd
	${MKDIR} ${STAGEDIR}${PREFIX}/share/man/man1
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/kwd.1 ${STAGEDIR}${PREFIX}/share/man/man1/
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/kubectl-kwd.1 ${STAGEDIR}${PREFIX}/share/man/man1/
	${MKDIR} ${STAGEDIR}${PREFIX}/share/doc/kwd
	${INSTALL_DATA} ${WRKDIR}/share/doc/kwd/LICENSE ${STAGEDIR}${PREFIX}/share/doc/kwd/
	${MKDIR} ${STAGEDIR}${PREFIX}/share/examples/kwd
	${INSTALL_DATA} ${WRKDIR}/share/examples/kwd/kwd.sample.yml ${STAGEDIR}${PREFIX}/share/examples/kwd/

.include <bsd.port.mk>
