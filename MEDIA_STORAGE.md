# Media Storage Configuration

This application uses **S3/MinIO** for media storage. S3-compatible storage is required for media download and upload features.

## Features

- **S3/MinIO Storage**: Cloud object storage for scalability
- **Transparent API**: Clean interface for storage operations
- **Automatic Initialization**: Storage is configured at startup
- **Server Proxy Support**: Option to proxy downloads for private buckets

## Configuration

### Environment Variables

Add the following variables to your `.env` file:

```env
# S3/MinIO Configuration (required for media storage)
S3_ENDPOINT=https://s3.amazonaws.com
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=your-access-key-id
S3_SECRET_ACCESS_KEY=your-secret-access-key
S3_BUCKET=whatsapp-media
S3_FORCE_PATH_STYLE=false
S3_PUBLIC_URL=
S3_USE_SERVER_PROXY=false
```

If S3 is not configured, media download/upload features will be disabled (the application will still work for text messages).

### Configuration Options

- **S3_ENDPOINT**: The S3-compatible endpoint URL
  - For AWS S3: Use `https://s3.amazonaws.com` (or region-specific endpoint like `https://s3.us-east-1.amazonaws.com`)
  - For MinIO: Use your MinIO server URL (e.g., `https://minio.example.com`)
  - For DigitalOcean Spaces: Use `https://<region>.digitaloceanspaces.com`

- **S3_REGION**: AWS region (e.g., `us-east-1`, `eu-west-1`)

- **S3_ACCESS_KEY_ID**: Your S3/MinIO access key

- **S3_SECRET_ACCESS_KEY**: Your S3/MinIO secret key

- **S3_BUCKET**: Name of the bucket to store media files
  - **IMPORTANT**: Bucket must be created manually before starting the application

- **S3_FORCE_PATH_STYLE**:
  - Set to `true` for MinIO or path-style URLs
  - Set to `false` for AWS S3 virtual-hosted-style URLs

- **S3_PUBLIC_URL**: (Optional) Custom public URL for accessing files
  - Leave empty to use default S3 URLs
  - Set to your CDN or custom domain if configured

- **S3_USE_SERVER_PROXY**:
  - Set to `false` for public buckets (direct URLs, faster)
  - Set to `true` for private buckets (server proxies downloads with authentication)

## Example Configurations

### AWS S3

```env
S3_ENDPOINT=https://s3.amazonaws.com
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
S3_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
S3_BUCKET=my-whatsapp-media
S3_FORCE_PATH_STYLE=false
S3_USE_SERVER_PROXY=false
```

### MinIO (Self-hosted)

```env
S3_ENDPOINT=https://minio.example.com
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=minioadmin
S3_SECRET_ACCESS_KEY=minioadmin
S3_BUCKET=whatsapp-media
S3_FORCE_PATH_STYLE=true
S3_USE_SERVER_PROXY=false
```

### DigitalOcean Spaces

```env
S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
S3_REGION=nyc3
S3_ACCESS_KEY_ID=your-spaces-key
S3_SECRET_ACCESS_KEY=your-spaces-secret
S3_BUCKET=whatsapp-media
S3_FORCE_PATH_STYLE=false
S3_PUBLIC_URL=https://whatsapp-media.nyc3.cdn.digitaloceanspaces.com
S3_USE_SERVER_PROXY=false
```

## Bucket Access Modes

### Public Bucket (Recommended for simplicity)

For public bucket access, make your bucket publicly readable:

```bash
# Using MinIO client
mc anonymous set download s3/whatsapp-media
```

Then set:
```env
S3_USE_SERVER_PROXY=false
```

Media URLs will be direct S3 URLs that clients can access without authentication.

### Private Bucket

For private bucket access (more secure):

```env
S3_USE_SERVER_PROXY=true
```

The server will proxy media downloads through the `/media/download/:deviceid/:chatjid/:filename` endpoint, adding authentication automatically.

## Troubleshooting

See [S3_TROUBLESHOOTING.md](./S3_TROUBLESHOOTING.md) for common issues and solutions.
