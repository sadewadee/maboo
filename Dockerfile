# Stage 1: Build PHP with extensions
FROM alpine:3.20 AS php-builder

ARG PHP_VERSION=8.3

# Install build dependencies
RUN apk add --no-cache \
    build-base autoconf bison re2c \
    libxml2-dev openssl-dev curl-dev \
    libpng-dev libjpeg-turbo-dev freetype-dev libwebp-dev \
    imagemagick-dev onig-dev icu-dev \
    libzip-dev libxslt-dev gmp-dev \
    openldap-dev libmemcached-dev hiredis-dev \
    libsodium-dev libpq-dev argon2-dev \
    readline-dev bzip2-dev \
    wget tar

WORKDIR /build

# Download PHP source
RUN wget -q https://www.php.net/distributions/php-${PHP_VERSION}.tar.gz && \
    tar xzf php-${PHP_VERSION}.tar.gz && \
    rm php-${PHP_VERSION}.tar.gz && \
    mv php-${PHP_VERSION} php-src

# Configure and build PHP as shared library
WORKDIR /build/php-src

RUN ./configure \
    --prefix=/usr/local \
    --enable-embed=shared \
    --disable-cgi \
    --disable-phpdbg \
    --enable-maintainer-zts \
    --enable-pdo \
    --enable-pdo_mysql \
    --enable-mysqli \
    --enable-mysqlnd \
    --enable-mbstring \
    --enable-xml \
    --enable-dom \
    --enable-simplexml \
    --enable-tokenizer \
    --enable-ctype \
    --enable-session \
    --enable-fileinfo \
    --enable-zip \
    --enable-opcache \
    --with-openssl \
    --with-sodium \
    --with-zlib \
    --with-curl \
    --with-gd \
    --with-jpeg \
    --with-freetype \
    --with-webp

RUN make -j$(nproc) && make install

# Stage 2: Build Maboo with CGO
FROM golang:1.24-alpine AS go-builder

ARG PHP_VERSION=8.3

RUN apk add --no-cache build-base pkgconfig

# Copy PHP from builder
COPY --from=php-builder /usr/local/lib/libphp.so /usr/local/lib/
COPY --from=php-builder /usr/local/lib/php /usr/local/lib/php
COPY --from=php-builder /usr/local/include/php /usr/local/include/php
COPY --from=php-builder /usr/local/lib/pkgconfig /usr/local/lib/pkgconfig

# Ensure linker/runtime can find libphp during build
ENV LD_LIBRARY_PATH=/usr/local/lib

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build with CGO enabled
COPY . .
RUN CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags "-s -w" \
    -tags "php_embed" \
    -o /maboo ./cmd/maboo

# Stage 3: Runtime
FROM alpine:3.20

ARG PHP_VERSION=8.3

# Ensure runtime loader can resolve libphp.so
ENV LD_LIBRARY_PATH=/usr/local/lib

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates tzdata \
    libxml2 openssl curl \
    libpng libjpeg-turbo freetype libwebp \
    imagemagick onig icu-libs \
    libzip libxslt gmp \
    openldap libmemcached hiredis \
    libsodium libpq argon2 \
    readline bzip2

# Copy PHP runtime
COPY --from=php-builder /usr/local/lib/libphp.so /usr/local/lib/
COPY --from=php-builder /usr/local/lib/php /usr/local/lib/php

# Copy Maboo binary
COPY --from=go-builder /maboo /usr/local/bin/maboo

# Copy config
COPY maboo.yaml.example /etc/maboo/maboo.yaml

# Create non-root user and directories
RUN adduser -D -u 1000 maboo && \
    mkdir -p /app /var/lib/maboo/certs && \
    chown -R maboo:maboo /app /var/lib/maboo

USER maboo
WORKDIR /app

EXPOSE 8080 8443

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["maboo"]
CMD ["serve", "/etc/maboo/maboo.yaml"]
