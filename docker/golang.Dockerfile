############################
# STEP 1 build executable binary
############################
FROM golang:1.27.4-alpine3.24 AS builder
RUN apk upgrade --no-cache && apk add --no-cache gcc musl-dev gcompat
WORKDIR /whatsapp
COPY ./src .

# Fetch dependencies.
RUN go mod download
# Build the binary with optimizations
RUN go build -a -ldflags="-w -s" -o /app/whatsapp

#############################
## STEP 2 build a smaller image
#############################
FROM alpine:3.24.3
# Link the GHCR package to its source repo so the repository's GITHUB_TOKEN
# inherits push access (avoids manual "Manage Actions access" UI grants).
LABEL org.opencontainers.image.source=https://github.com/hbinduni/go-whatsapp-web-multidevice
# Minimum patched versions also invalidate older cached package-install layers.
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates ffmpeg tzdata 'libexpat>=2.10.0-r0' 'zlib>=1.4-r0'
ENV TZ=UTC
WORKDIR /app
# Copy compiled from builder.
COPY --from=builder /app/whatsapp /app/whatsapp
# Run the binary.
ENTRYPOINT ["/app/whatsapp"]

CMD [ "rest" ]
