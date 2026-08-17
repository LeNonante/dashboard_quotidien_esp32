FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o dashboard-server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/dashboard-server .
COPY template/ ./template/
EXPOSE 8081
CMD ["./dashboard-server"]