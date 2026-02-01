FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags='-s -w -extldflags "-static"' -o /usr/bin/journald-plus .

FROM alpine:3.21
COPY --from=builder /usr/bin/journald-plus /usr/bin/journald-plus
