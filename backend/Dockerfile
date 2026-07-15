FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/workhub .

FROM denoland/deno:bin AS deno

# Debian, not alpine: yt-dlp's JS "n" signature challenge solver is only
# fully supported on deno (node support is best-effort and fails on some
# challenges), and deno's official binary is glibc-linked — doesn't run on
# alpine/musl without extra compat shims.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ffmpeg python3 python3-pip && \
    pip install --no-cache-dir --break-system-packages yt-dlp && \
    rm -rf /var/lib/apt/lists/*
COPY --from=deno /deno /usr/local/bin/deno
WORKDIR /app
COPY --from=build /out/workhub .
EXPOSE 3001
ENTRYPOINT ["./workhub"]
