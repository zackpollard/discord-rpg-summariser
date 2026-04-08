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
RUN git clone --depth 1 --branch v1.8.4 https://github.com/ggerganov/whisper.cpp _deps/whisper.cpp \
    && cmake -B _deps/whisper.cpp/build -S _deps/whisper.cpp \
        -DCMAKE_BUILD_TYPE=Release \
        -DGGML_NATIVE=OFF \
        -DWHISPER_BUILD_EXAMPLES=OFF \
        -DWHISPER_BUILD_TESTS=OFF \
    && cmake --build _deps/whisper.cpp/build --config Release -j$(nproc)

# Download Go dependencies
COPY go.mod go.sum ./
COPY _deps/discordgo-fork/ _deps/discordgo-fork/
RUN go mod download

# Collect sherpa-onnx shared libraries for the current architecture
RUN SHERPA_MOD=/go/pkg/mod/github.com/k2-fsa/sherpa-onnx-go-linux@v1.12.30/lib \
    && ARCH=$(uname -m) \
    && case "$ARCH" in \
         x86_64)  SHERPA_ARCH=x86_64-unknown-linux-gnu ;; \
         aarch64) SHERPA_ARCH=aarch64-unknown-linux-gnu ;; \
         armv7l)  SHERPA_ARCH=arm-unknown-linux-gnueabihf ;; \
         *)       echo "Unsupported arch: $ARCH" && exit 1 ;; \
       esac \
    && mkdir -p /sherpa-libs \
    && cp "$SHERPA_MOD/$SHERPA_ARCH"/*.so /sherpa-libs/

# Build the Go binary
COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/

ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/app/_deps/whisper.cpp/build/src -L/app/_deps/whisper.cpp/build/ggml/src -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++ -fopenmp"
ENV CGO_CFLAGS="-I/app/_deps/whisper.cpp/include -I/app/_deps/whisper.cpp/ggml/include"

ARG VERSION=dev
RUN go build -tags nolibopusfile -ldflags "-X main.version=${VERSION}" -o /bot ./cmd/bot/

# Stage 3: Python TTS venv (built on same base as runtime for binary compat)
FROM python:3.12-slim-bookworm AS python-tts

RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*

# Create a portable venv
RUN python3 -m venv /opt/tts-venv

# Install PyTorch CPU
RUN /opt/tts-venv/bin/pip install --no-cache-dir \
    torch==2.6.0+cpu torchaudio==2.6.0+cpu \
    --index-url https://download.pytorch.org/whl/cpu

# Clone ZipVoice
RUN git clone --depth 1 https://github.com/k2-fsa/ZipVoice.git /opt/tts-venv/ZipVoice

# Install piper_phonemize from custom index
RUN /opt/tts-venv/bin/pip install --no-cache-dir piper_phonemize \
    -f https://k2-fsa.github.io/icefall/piper_phonemize.html

# Install remaining ZipVoice requirements + sentencepiece for bpe.vocab generation
RUN /opt/tts-venv/bin/pip install --no-cache-dir \
    -r /opt/tts-venv/ZipVoice/requirements.txt sentencepiece

# Stage 4: Runtime
FROM node:22-slim AS runtime

# Install runtime deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libopus0 libgomp1 \
    libsndfile1 ffmpeg \
    && rm -rf /var/lib/apt/lists/* \
    && npm install -g @anthropic-ai/claude-code \
    && npm cache clean --force

WORKDIR /app

# Copy whisper shared libraries
COPY --from=backend /app/_deps/whisper.cpp/build/src/libwhisper.so* /usr/lib/
COPY --from=backend /app/_deps/whisper.cpp/build/ggml/src/libggml*.so* /usr/lib/

# Copy sherpa-onnx shared libraries (speaker diarization)
COPY --from=backend /sherpa-libs/*.so /usr/lib/
RUN ldconfig

# Copy the pre-built Python TTS venv (includes Python 3.12 binary + all packages)
COPY --from=python-tts /opt/tts-venv /app/.venv

# Copy the Python 3.12 runtime from the build stage (bookworm ships 3.11
# but the venv packages need 3.12)
COPY --from=python-tts /usr/local/lib/libpython3.12* /usr/local/lib/
COPY --from=python-tts /usr/local/lib/python3.12 /usr/local/lib/python3.12
COPY --from=python-tts /usr/local/bin/python3.12 /usr/local/bin/python3.12
RUN ldconfig \
    && rm -f /app/.venv/bin/python /app/.venv/bin/python3 \
    && ln -s /usr/local/bin/python3.12 /app/.venv/bin/python \
    && ln -s /usr/local/bin/python3.12 /app/.venv/bin/python3

# Copy application
COPY --from=backend /bot .
COPY --from=frontend /app/web/build web/build
COPY migrations/ migrations/
COPY scripts/ scripts/

EXPOSE 8080

# Entrypoint script creates /data subdirectories and the claude-cli symlink
# at runtime (after the volume is mounted), then execs the bot.
RUN printf '#!/bin/sh\nmkdir -p /data/audio /data/models /data/claude\n[ -e /root/.claude ] || ln -s /data/claude /root/.claude\nexec "$@"\n' > /app/entrypoint.sh \
    && chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./bot", "-config", "/data/config.yaml"]
