FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/pramool-wallet-service .

FROM alpine:3.20
RUN adduser -D -H -s /sbin/nologin appuser
WORKDIR /app
COPY --from=builder /out/pramool-wallet-service ./pramool-wallet-service
USER appuser
EXPOSE 3102
CMD ["./pramool-wallet-service"]
