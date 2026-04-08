FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY configs ./configs

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
	go build -trimpath \
	-ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
	-o /out/subconverter ./cmd/subconverter

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd --system --create-home --home-dir /app --uid 10001 appuser \
	&& mkdir -p /config /app/configs \
	&& chown -R appuser:appuser /app /config

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /out/subconverter /app/subconverter
COPY --chown=appuser:appuser configs /app/configs

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/subconverter"]
CMD ["-config", "/config/config.yaml", "-listen", ":8080"]
