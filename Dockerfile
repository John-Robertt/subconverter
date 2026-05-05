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
	-tags webui \
	-ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
	-o /out/subconverter ./cmd/subconverter

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/subconverter /app/subconverter
COPY configs /app/configs

WORKDIR /app

ENV SUBCONVERTER_LISTEN=:8080
ENV SUBCONVERTER_AUTH_STATE=/auth/auth.json

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=20 \
	CMD ["/app/subconverter", "-healthcheck"]

ENTRYPOINT ["/app/subconverter"]
CMD ["-config", "/config/config.yaml"]
