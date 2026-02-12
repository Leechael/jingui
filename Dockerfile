FROM golang:1.24-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETOS TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags "-X github.com/aspect-build/jingui/internal/version.Version=${VERSION} -X github.com/aspect-build/jingui/internal/version.GitCommit=${COMMIT}" \
      -o /out/jingui ./cmd/jingui

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags "-X github.com/aspect-build/jingui/internal/version.Version=${VERSION} -X github.com/aspect-build/jingui/internal/version.GitCommit=${COMMIT}" \
      -o /out/jingui-server ./cmd/jingui-server

# ── Server image ───────────────────────────────────────────────────────
FROM alpine:3.21 AS server
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/jingui-server /usr/local/bin/jingui-server

ENV JINGUI_MASTER_KEY=""
ENV JINGUI_ADMIN_TOKEN=""
ENV JINGUI_DB_PATH="/data/jingui.db"
ENV JINGUI_LISTEN_ADDR=":8080"
ENV JINGUI_BASE_URL=""

VOLUME /data
EXPOSE 8080

ENTRYPOINT ["jingui-server"]

# ── Client image ───────────────────────────────────────────────────────
FROM alpine:3.21 AS client
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/jingui /usr/local/bin/jingui

ENV JINGUI_SERVER_URL=""

ENTRYPOINT ["jingui"]
