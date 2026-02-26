FROM rust:1.85-alpine AS dcap-builder
ARG DCAP_QVL_REF=c201e5e2b312
WORKDIR /src
RUN apk add --no-cache git musl-dev
RUN git clone https://github.com/Phala-Network/dcap-qvl.git . \
  && git checkout ${DCAP_QVL_REF}
RUN cargo build --release --features go

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache build-base
COPY --from=dcap-builder /src/target/release/libdcap_qvl.a /usr/local/lib/libdcap_qvl.a

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG TARGETOS TARGETARCH
ARG GO_TAGS=ratls
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS=-L/usr/local/lib

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -tags "${GO_TAGS}" \
      -ldflags "-X github.com/aspect-build/jingui/internal/version.Version=${VERSION} -X github.com/aspect-build/jingui/internal/version.GitCommit=${COMMIT}" \
      -o /out/jingui ./cmd/jingui

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -tags "${GO_TAGS}" \
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
