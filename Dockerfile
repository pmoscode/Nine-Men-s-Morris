FROM golang:1.25-alpine AS builder
WORKDIR /app

RUN apk add --no-cache curl libstdc++ libgcc

COPY . .

# TailwindCSS herunterladen, CSS bauen (Architektur automatisch erkennen)
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64)        TW_ARCH="x64" ;; \
      arm64|aarch64) TW_ARCH="arm64" ;; \
      *)             TW_ARCH="x64" ;; \
    esac && \
    curl -fsSLo /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-${TW_ARCH}-musl" \
    && chmod +x /usr/local/bin/tailwindcss \
    && tailwindcss -i static/css/input.css -o static/css/app.css --minify
RUN go build -o muehle .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/muehle .
ARG PORT=8080
ENV PORT=${PORT}
EXPOSE ${PORT}
CMD ["./muehle"]
