FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /maboo ./cmd/maboo

FROM php:8.3-cli-bookworm
COPY --from=builder /maboo /usr/local/bin/maboo
COPY php-sdk /opt/maboo/php-sdk
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["maboo"]
CMD ["serve"]
