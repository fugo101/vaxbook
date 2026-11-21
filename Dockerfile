# syntax=docker/dockerfile:1
# Single static binary on distroless, non-root (SAD §10).

FROM golang:1.27.4 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/ apps/
COPY contracts/ contracts/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/vaxbook ./apps/api/cmd/vaxbook

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/vaxbook /vaxbook
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s CMD ["/vaxbook", "healthcheck"]
ENTRYPOINT ["/vaxbook"]
