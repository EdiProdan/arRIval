FROM node:22-alpine AS ui-builder

WORKDIR /ui

COPY web/realtime-ui/package.json ./
RUN npm install

COPY web/realtime-ui ./
RUN npm run build

FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /ui/dist ./web/realtime-ui/dist

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ingester ./cmd/ingester && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/processor ./cmd/processor && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/aggregator ./cmd/aggregator && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/realtime ./cmd/realtime && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/staticsync ./cmd/staticsync

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/ingester /usr/local/bin/ingester
COPY --from=builder /out/processor /usr/local/bin/processor
COPY --from=builder /out/aggregator /usr/local/bin/aggregator
COPY --from=builder /out/realtime /usr/local/bin/realtime
COPY --from=builder /out/staticsync /usr/local/bin/staticsync
COPY --from=ui-builder /ui/dist /app/web/realtime-ui/dist
