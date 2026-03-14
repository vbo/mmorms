# Build overlord
FROM golang:1.22-alpine AS overlord-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY overlord/ ./overlord/
RUN CGO_ENABLED=0 go build -o overlord ./overlord

# Build mmorms
FROM golang:1.22-alpine AS mmorms-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go network.go bot.go ./
COPY botai/ ./botai/
RUN CGO_ENABLED=0 go build -o mmorms .

# Runtime
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=overlord-builder /app/overlord .
COPY --from=mmorms-builder /app/mmorms .
COPY public/ ./public/
COPY scripts/entrypoint.sh ./
RUN chmod +x entrypoint.sh
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["./entrypoint.sh"]
