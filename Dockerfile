# kwd — minimal runtime image (build from repo root: make docker-build)
# Final stage uses distroless (no Alpine/BusyBox) so CVEs in wget/busybox from
# minimal Alpine bases do not apply; CA certs are included in distroless static.
FROM golang:1.26.6-alpine3.24 AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILDDATE=unknown
ARG BRANCH=unknown
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY configs/ ./configs/
RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w -X github.com/hrodrig/kwd/internal/cli.Version=${VERSION} -X github.com/hrodrig/kwd/internal/cli.Commit=${COMMIT} -X github.com/hrodrig/kwd/internal/cli.BuildDate=${BUILDDATE} -X github.com/hrodrig/kwd/internal/cli.Branch=${BRANCH}" \
	-o /kwd ./cmd/kwd

FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.title="kwd"
LABEL org.opencontainers.image.description="Kubernetes workload watchdog"
LABEL org.opencontainers.image.source="https://github.com/hrodrig/kwd"
COPY --from=build /kwd /usr/local/bin/kwd
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kwd"]
