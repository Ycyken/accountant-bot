FROM debian:bookworm-slim AS whisper
RUN apt update && apt install -y --no-install-recommends ca-certificates build-essential cmake git wget \
  && git clone --depth=1 https://github.com/ggml-org/whisper.cpp.git /opt/whisper \
  && cd /opt/whisper \
  && cmake -B build -DBUILD_SHARED_LIBS=OFF -DBUILD_SHARED_LIBS=OFF \
                                                   -DCMAKE_EXE_LINKER_FLAGS="-static -static-libgcc -static-libstdc++" \
                                                   -DGGML_STATIC=ON \
  && cmake --build build -j --config Release \
  && cp build/bin/whisper-cli /usr/local/bin/ \
  && rm -rf /var/lib/apt/lists/* /opt/whisper

FROM golang:1.24-bookworm AS builder
WORKDIR /app

COPY ../go.mod go.sum vendor ./

COPY .. .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make init && make build-prod

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates ffmpeg
COPY ../cfg/config.toml /app/cfg/config.toml
COPY --from=builder /app/saldo .
COPY --from=builder /app/cfg/config.toml /app/cfg/config.toml
COPY --from=whisper /usr/local/bin/whisper-cli /usr/local/bin/whisper-cli

CMD ["./saldo", "-config=cfg/config.toml", "-dev"]