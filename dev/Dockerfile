# Conspectus - multi-stage build.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
# TARGETOS/TARGETARCH are set by BuildKit for multi-arch builds; empty means native.
ARG TARGETOS
ARG TARGETARCH
# VERSION/COMMIT/DATE come from CI (release.yml). Left at the default (a plain
# `docker compose build`), the build stamps <tag>-dev-[<branch>-]<shorthash>[-dirty]
# from git instead, falling back to the hardcoded version when no tag is reachable.
ARG VERSION=2.0.0-dev
ARG COMMIT=unknown
ARG DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN if [ "${VERSION}" = "2.0.0-dev" ]; then \
      apk add --no-cache git \
      && git config --global --add safe.directory /src \
      && git fetch --tags origin >/dev/null 2>&1 || : \
      && BASE="$(git describe --tags --abbrev=0 --match='[0-9]*' 2>/dev/null || echo 2.0.0)" \
      && SHA="$(git rev-parse --short HEAD 2>/dev/null || echo norev)" \
      && git update-index --refresh >/dev/null 2>&1 || : \
      && if [ "$SHA" != "norev" ] && ! git diff-index --quiet HEAD 2>/dev/null; then SHA="${SHA}-dirty"; fi \
      && BRANCH="$(git branch --show-current 2>/dev/null || :)" \
      && STAMP="$SHA" \
      && if [ -n "$BRANCH" ]; then STAMP="${BRANCH}-${STAMP}"; fi \
      && VERSION="${BASE}-dev-${STAMP}" \
      && if [ "${COMMIT}" = "unknown" ]; then COMMIT="${SHA}"; fi; \
    fi \
 && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /out/conspectus ./cmd/conspectus

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata wget \
 && addgroup -S conspectus && adduser -S conspectus -G conspectus
COPY --from=build /out/conspectus /usr/local/bin/conspectus
# Everything ships inside the binary except the web templates/assets (and
# the optional migration-dir drop-ins): web/ is read from disk at runtime so
# it can be edited in place without a rebuild.
COPY --from=build /src/web        /app/web
COPY --from=build /src/migrations /app/migrations
USER conspectus
# Migrations ship inside the image at /app/migrations - bind-mount over it
# to override (the dev stack does); templates/assets ship at /app/web (the
# app's built-in default) and need no volume - the database is the only
# persistent state.
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["conspectus"]
