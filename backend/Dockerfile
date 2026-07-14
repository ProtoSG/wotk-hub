FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/workhub .

FROM alpine:3.20
# ffmpeg + nodejs are yt-dlp's runtime deps (audio extraction, and YouTube's
# JS signature challenge — apk's yt-dlp lags releases, so it's installed via
# pip instead to stay current against YouTube's anti-bot changes).
RUN apk add --no-cache ca-certificates ffmpeg nodejs python3 py3-pip && \
    pip install --no-cache-dir --break-system-packages yt-dlp
WORKDIR /app
COPY --from=build /out/workhub .
EXPOSE 3001
ENTRYPOINT ["./workhub"]
