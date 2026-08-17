FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o dashboard-server .

FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    tzdata && \
    ln -s /usr/bin/chromium-browser /usr/bin/google-chrome

COPY --from=builder /app/dashboard-server .
COPY template/ ./template/
EXPOSE 8081
CMD ["./dashboard-server"]