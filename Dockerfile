# syntax=docker/dockerfile:1

# Versi Go untuk build. Boleh dinaikkan: docker build --build-arg GO_VERSION=1.25 .
ARG GO_VERSION=1.24

# ---- Tahap 1: build ---------------------------------------------------------
# --platform=$BUILDPLATFORM: compiler berjalan native di mesin build (cepat),
# lalu cross-compile ke TARGETOS/TARGETARCH (amd64 atau arm64).
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

# Salin go.mod dulu agar layer dependensi ter-cache selama go.mod tidak berubah.
# (Jika nanti ada dependensi, salin juga go.sum: COPY go.mod go.sum ./)
COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

# ---- Tahap 2: image akhir ---------------------------------------------------
# distroless: tanpa shell dan package manager, jadi kecil dan minim celah keamanan.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /app/api

# UID numerik agar Kubernetes bisa memverifikasi runAsNonRoot.
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api"]
