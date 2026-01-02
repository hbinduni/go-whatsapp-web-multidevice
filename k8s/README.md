# WhatsApp Multi-Client K8s Deployment

This directory contains Kubernetes manifests for deploying multiple isolated WhatsApp API instances.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        K8s Cluster                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Namespace: whatsapp-01     Namespace: whatsapp-02    ...       │
│  ┌────────────────────┐    ┌────────────────────┐              │
│  │  WhatsApp API Pod  │    │  WhatsApp API Pod  │              │
│  │  (10 phones)       │    │  (10 phones)       │              │
│  └─────────┬──────────┘    └─────────┬──────────┘              │
│            │                         │                          │
│  ┌─────────▼──────────┐    ┌─────────▼──────────┐              │
│  │  PostgreSQL Pod    │    │  PostgreSQL Pod    │              │
│  │  (instance-01-db)  │    │  (instance-02-db)  │              │
│  └────────────────────┘    └────────────────────┘              │
│                                                                  │
│  Each instance is completely isolated with its own:             │
│  - Namespace                                                     │
│  - PostgreSQL database                                          │
│  - PersistentVolumeClaim                                        │
│  - Ingress endpoint                                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
k8s/
├── base/                    # Base templates (shared configuration)
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ingress.yaml
├── overlays/                # Per-instance configurations
│   ├── instance-01/
│   │   ├── kustomization.yaml
│   │   ├── namespace-patch.yaml
│   │   ├── secret-patch.yaml
│   │   ├── ingress-patch.yaml
│   │   └── postgres.yaml
│   ├── instance-02/
│   └── ... (up to instance-10)
├── generate-instances.sh    # Script to generate instance configs
├── deploy-all.sh           # Script to deploy all instances
└── README.md
```

## Quick Start

### 1. Generate Instance Configurations

```bash
# Generate 10 instances with 10 phones each (100 phones total)
./generate-instances.sh 10 10

# Or custom: 5 instances with 20 phones each
./generate-instances.sh 5 20
```

### 2. Configure Each Instance

Edit the secret patches for each instance:

```bash
# Edit instance 01
vim overlays/instance-01/secret-patch.yaml
```

**Required changes per instance:**
- `DATABASE_URL` - Point to your PostgreSQL (or use included StatefulSet)
- `AUTH_SECRET` - Generate unique: `openssl rand -base64 32`
- `AUTH_PASSWORD_HASH` - Generate: `htpasswd -nbBC 10 "" yourpassword | tr -d ':\n'`
- `WHATSAPP_CLIENTS` - Your actual phone numbers
- `S3_*` - Your S3/MinIO credentials (in base/secret.yaml)

### 3. Deploy

```bash
# Deploy single instance
kubectl apply -k overlays/instance-01

# Deploy all instances
./deploy-all.sh

# Or manually
for i in $(seq -w 1 10); do
  kubectl apply -k overlays/instance-$i
done
```

### 4. Verify Deployment

```bash
# Check all namespaces
kubectl get ns | grep whatsapp

# Check pods in instance-01
kubectl get pods -n whatsapp-01

# Check logs
kubectl logs -n whatsapp-01 -l app.kubernetes.io/name=whatsapp-api -f
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_CLIENTS` | Max phones per instance | 10 |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `AUTH_SECRET` | JWT signing secret | - |
| `WHATSAPP_CLIENTS` | Comma-separated phone numbers | - |

### Resource Limits

Default per instance:
- **WhatsApp API**: 256Mi-512Mi RAM, 100m-500m CPU
- **PostgreSQL**: 256Mi-512Mi RAM, 100m-500m CPU
- **Storage**: 10Gi per PostgreSQL instance

Adjust in `base/deployment.yaml` and instance `postgres.yaml`.

## Scaling

### Horizontal Scaling (More Instances)

```bash
# Generate 20 instances instead of 10
./generate-instances.sh 20 10
```

### Vertical Scaling (More Phones per Instance)

1. Update `MAX_CLIENTS` in `base/configmap.yaml`
2. Update phone lists in each instance's `secret-patch.yaml`

```yaml
# base/configmap.yaml
data:
  MAX_CLIENTS: "20"  # Increase from 10 to 20
```

## Ingress Configuration

Each instance gets its own ingress endpoint:
- Instance 01: `whatsapp-01.example.com`
- Instance 02: `whatsapp-02.example.com`
- ...

Update the host in `overlays/instance-XX/ingress-patch.yaml`.

### Single Domain with Path-Based Routing

If you prefer one domain:

```yaml
# overlays/instance-01/ingress-patch.yaml
spec:
  rules:
    - host: whatsapp.example.com
      http:
        paths:
          - path: /instance-01
            pathType: Prefix
            backend:
              service:
                name: instance-01-whatsapp-api
                port:
                  name: http
```

## External PostgreSQL

To use an external PostgreSQL instead of the included StatefulSet:

1. Remove `postgres.yaml` from the instance overlay
2. Update `kustomization.yaml`:
   ```yaml
   resources:
     - ../../base
     # - postgres.yaml  # Remove this line
   ```
3. Update `secret-patch.yaml` with external DB URL:
   ```yaml
   DATABASE_URL: "postgresql://user:pass@external-db.example.com:5432/whatsapp_01"
   ```

## Monitoring

### Health Checks

Each instance exposes:
- Liveness: `GET /` (pod is alive)
- Readiness: `GET /` (pod is ready to serve)

### Logs

```bash
# Stream logs from all instances
for i in $(seq -w 1 10); do
  kubectl logs -n whatsapp-$i -l app.kubernetes.io/name=whatsapp-api -f &
done
```

## Troubleshooting

### Pod not starting

```bash
kubectl describe pod -n whatsapp-01 -l app.kubernetes.io/name=whatsapp-api
kubectl logs -n whatsapp-01 -l app.kubernetes.io/name=whatsapp-api --previous
```

### Database connection issues

```bash
# Check PostgreSQL is running
kubectl get pods -n whatsapp-01 -l app.kubernetes.io/name=postgres

# Test connection from API pod
kubectl exec -n whatsapp-01 -it deploy/instance-01-whatsapp-api -- \
  psql "postgresql://whatsapp:whatsapp@instance-01-postgres:5432/whatsapp" -c "SELECT 1"
```

### Storage issues

```bash
# Check PVC status
kubectl get pvc -n whatsapp-01
```

## Cleanup

```bash
# Delete single instance
kubectl delete -k overlays/instance-01

# Delete all instances
for i in $(seq -w 1 10); do
  kubectl delete -k overlays/instance-$i
done
```
