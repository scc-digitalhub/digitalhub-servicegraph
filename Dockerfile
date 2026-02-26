# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

# ─── Build stage ─────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Cache module downloads separately from source code
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o servicegraph .

# ─── Final stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /build/servicegraph /app/servicegraph

# Port the health server listens on. Override with -e HEALTH_PORT=<n> if needed.
ENV HEALTH_PORT=8090

# The pipeline YAML must be mounted at runtime, e.g.:
#   docker run --rm -v ./pipeline.yaml:/app/pipeline.yaml servicegraph /app/pipeline.yaml
# Expose common source port and health port (override as needed)
EXPOSE 8080
EXPOSE 8090

# The same binary handles the healthcheck probe via --healthcheck flag.
# No extra binary or shell needed in the distroless image.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/servicegraph", "--healthcheck"]

ENTRYPOINT ["/app/servicegraph"]
