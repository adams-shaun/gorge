# The gorge demo image: gorged with the web client embedded.
#
#   docker build -f deploy/docker/Dockerfile --build-arg FORGE_REF=<sha> -t gorge/gorged-demo .
#   make demo-image                      (passes FORGE_REF from the Makefile)
#
# Three build stages and two runtime targets:
#   web    builds the Svelte client into server/gorged/webdist
#   gobase compiles gorged and forgec with no client (a Go-only check)
#   gorged the server image (target of the default build)
#   proxy  the nginx front proxy (a separate target; compose builds both)
#
# The Forge card scripts are GPL-3.0 and are never copied into an image: the
# gorged container fetches them at first start into a volume (entrypoint.sh).

FROM node:24.11.1-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
# vite.config.ts writes the bundle to ../server/gorged/webdist.
RUN npm run build

FROM golang:1.25.8-bookworm AS gobase
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
WORKDIR /src
# The module has no third-party dependencies, so there is nothing to download.
COPY . .
# Compile gorged once with the empty client so a Go-only build checks the tree
# and warms the build cache; the real build below only recompiles the embed.
RUN go build -o /out/forgec ./cmd/forgec \
 && go build -o /tmp/gorged-noclient ./cmd/gorged

FROM gobase AS gobuild
COPY --from=web /src/server/gorged/webdist/ server/gorged/webdist/
RUN go build -o /out/gorged ./cmd/gorged

FROM debian:bookworm-20251020-slim AS gorged
# git: forgec fetch sparse-clones the card scripts. ca-certificates: that clone
# and the card-art requests are HTTPS.
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 gorge \
 && useradd --uid 10001 --gid 10001 --no-create-home --shell /usr/sbin/nologin gorge \
 && mkdir -p /cards /data /art /feedback \
 && chown 10001:10001 /cards /data /art /feedback
COPY --from=gobuild /out/gorged /out/forgec /usr/local/bin/
# The demo decks are the repository's own lists, not Forge data.
COPY testkit/testutil/decks /app/decks
COPY deploy/docker/entrypoint.sh /usr/local/bin/entrypoint.sh
# FORGE_REF is the corpus commit this build is pinned to (the Makefile's pin).
ARG FORGE_REF
ENV FORGE_REF=${FORGE_REF} HOME=/tmp
USER 10001:10001
VOLUME ["/cards", "/data", "/art", "/feedback"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["gorged", "-addr", ":8080", "-cards", "/cards", "-decks", "/app/decks", "-dir", "/data", "-art-dir", "/art", "-feedback", "/feedback", "-vsbot", "-events", "-manabrew"]

FROM nginxinc/nginx-unprivileged:1.28.1-alpine AS proxy
COPY deploy/docker/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
