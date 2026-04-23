FROM golang:1.23-alpine AS builder
WORKDIR /app

RUN apk add --no-cache curl

# TailwindCSS herunterladen und CSS bauen
RUN curl -sLo /usr/local/bin/tailwindcss \
    https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    && chmod +x /usr/local/bin/tailwindcss

COPY . .
RUN tailwindcss -i static/css/input.css -o static/css/app.css --minify
RUN go build -o muehle ./...

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/muehle .
EXPOSE 8080
CMD ["./muehle"]
