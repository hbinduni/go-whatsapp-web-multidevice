############################
# Backup image
# Pre-installed with sqlite, tzdata, MinIO client, and kubectl
# Used for both sidecar and centralized CronJob backup
############################
FROM alpine:3.20

# Install required tools
RUN apk add --no-cache sqlite tzdata curl \
    && curl -sL https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc \
    && chmod +x /usr/local/bin/mc \
    && curl -sL "https://dl.k8s.io/release/$(curl -sL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl \
    && rm -rf /var/cache/apk/*

# Default timezone (can be overridden by TZ env var)
ENV TZ=Asia/Jakarta

# Create scripts directory
RUN mkdir -p /scripts

WORKDIR /app

# Default command (overridden by k8s command)
CMD ["/bin/sh"]
