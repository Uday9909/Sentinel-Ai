# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ingestion-server .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
EXPOSE 8080
COPY --from=builder /app/ingestion-server /ingestion-server
CMD ["/ingestion-server"]
