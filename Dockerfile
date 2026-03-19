# Stage 1: Build Svelte frontend
FROM node:22-slim AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build whisper.cpp and Go binary
FROM golang:1.23-bookworm AS backend
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake build-essential git pkg-config libopus-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Clone and build whisper.cpp
RUN git clone --depth 1 https://github.com/ggerganov/whisper.cpp _deps/whisper.cpp \
    && cmake -B _deps/whisper.cpp/build -S _deps/whisper.cpp \
        -DCMAKE_BUILD_TYPE=Release \
        -DWHISPER_BUILD_EXAMPLES=OFF \
        -DWHISPER_BUILD_TESTS=OFF \
    && cmake --build _deps/whisper.cpp/build --config Release -j$(nproc)

# Download Go dependencies
COPY go.mod go.sum ./
COPY _deps/discordgo-fork/ _deps/discordgo-fork/
RUN go mod download

# Build the Go binary
COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/

ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/app/_deps/whisper.cpp/build/src -L/app/_deps/whisper.cpp/build/ggml/src -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -fopenmp"
ENV CGO_CFLAGS="-I/app/_deps/whisper.cpp/include -I/app/_deps/whisper.cpp/ggml/include"

RUN go build -tags nolibopusfile -o /bot ./cmd/bot/

# Stage 3: Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libopus0 libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy whisper shared libraries
COPY --from=backend /app/_deps/whisper.cpp/build/src/libwhisper.so* /usr/lib/
COPY --from=backend /app/_deps/whisper.cpp/build/ggml/src/libggml*.so* /usr/lib/
RUN ldconfig

# Copy application
COPY --from=backend /bot .
COPY --from=frontend /app/web/build web/build
COPY migrations/ migrations/

# Create directories for data
RUN mkdir -p data/audio models

EXPOSE 8080

ENTRYPOINT ["./bot"]
CMD ["-config", "/app/config.yaml"]
