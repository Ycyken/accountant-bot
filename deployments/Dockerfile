FROM debian:bookworm-slim AS whisper
RUN apt update && apt install -y build-essential cmake git wget \
  && git clone --depth=1 https://github.com/ggml-org/whisper.cpp.git /opt/whisper \
  && cd /opt/whisper &&  sh ./models/download-ggml-model.sh tiny \
  && cmake -B build && cmake --build build -j --config Release \
  && cp build/bin/whisper-cli /usr/local/bin/ \
  && rm -rf /var/lib/apt/lists/* /opt/whisper

FROM golang:1.24-bookworm AS builder
WORKDIR /app

COPY ../go.mod go.sum ./
RUN go mod download

COPY .. .
RUN make init
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make build

FROM debian:bookworm-slim
WORKDIR /app
RUN apt update && apt install -y ca-certificates ffmpeg
COPY --from=builder /app/saldo .
COPY --from=builder /app/cfg/config.toml /app/cfg/config.toml
COPY --from=whisper /usr/local/bin/whisper-cli /usr/local/bin/whisper-cli

CMD ["./saldo"]

