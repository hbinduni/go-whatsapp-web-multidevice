#!/bin/bash
# Generate K8s overlay configurations for multiple WhatsApp instances
# Usage: ./generate-instances.sh [num_instances] [phones_per_instance]

set -e

NUM_INSTANCES=${1:-10}
PHONES_PER_INSTANCE=${2:-10}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OVERLAYS_DIR="$SCRIPT_DIR/overlays"

echo "Generating $NUM_INSTANCES instances with $PHONES_PER_INSTANCE phones each..."

for i in $(seq -w 1 $NUM_INSTANCES); do
    INSTANCE_DIR="$OVERLAYS_DIR/instance-$i"
    mkdir -p "$INSTANCE_DIR"

    # Calculate phone number range
    START_PHONE=$((10#$i * PHONES_PER_INSTANCE - PHONES_PER_INSTANCE + 1))
    END_PHONE=$((10#$i * PHONES_PER_INSTANCE))

    # Generate comma-separated phone list (placeholder format)
    PHONES=""
    for p in $(seq $START_PHONE $END_PHONE); do
        PHONE=$(printf "628100000%04d" $p)
        if [ -z "$PHONES" ]; then
            PHONES="$PHONE"
        else
            PHONES="$PHONES,$PHONE"
        fi
    done

    # Generate unique auth secret
    AUTH_SECRET=$(openssl rand -base64 32 2>/dev/null || echo "REPLACE_WITH_UNIQUE_SECRET_$i")

    # Create kustomization.yaml
    cat > "$INSTANCE_DIR/kustomization.yaml" << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: whatsapp-$i

namePrefix: instance-$i-

resources:
  - ../../base
  - postgres.yaml

patches:
  - path: namespace-patch.yaml
  - path: secret-patch.yaml
  - path: ingress-patch.yaml

commonLabels:
  app.kubernetes.io/instance: instance-$i

commonAnnotations:
  whatsapp.io/instance-id: "$i"
EOF

    # Create namespace-patch.yaml
    cat > "$INSTANCE_DIR/namespace-patch.yaml" << EOF
apiVersion: v1
kind: Namespace
metadata:
  name: whatsapp-$i
EOF

    # Create secret-patch.yaml
    cat > "$INSTANCE_DIR/secret-patch.yaml" << EOF
apiVersion: v1
kind: Secret
metadata:
  name: whatsapp-secret
stringData:
  # Instance-specific database
  DATABASE_URL: "postgresql://whatsapp:whatsapp@instance-$i-postgres:5432/whatsapp?sslmode=disable"

  # Instance-specific auth secret
  AUTH_SECRET: "$AUTH_SECRET"

  # Instance-specific phone numbers ($PHONES_PER_INSTANCE phones)
  # Instance $i: phones $START_PHONE-$END_PHONE
  WHATSAPP_CLIENTS: "$PHONES"

  # Webhook endpoint
  WHATSAPP_WEBHOOK: "https://webhook.example.com/instance-$i"
EOF

    # Create ingress-patch.yaml
    cat > "$INSTANCE_DIR/ingress-patch.yaml" << EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: whatsapp-api
spec:
  rules:
    - host: whatsapp-$i.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: instance-$i-whatsapp-api
                port:
                  name: http
EOF

    # Create postgres.yaml (instance-specific PostgreSQL)
    cat > "$INSTANCE_DIR/postgres.yaml" << EOF
---
apiVersion: v1
kind: Secret
metadata:
  name: postgres-secret
  namespace: whatsapp-$i
type: Opaque
stringData:
  POSTGRES_USER: whatsapp
  POSTGRES_PASSWORD: whatsapp
  POSTGRES_DB: whatsapp
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: whatsapp-$i
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  # storageClassName: standard  # Uncomment and set your storage class
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: whatsapp-$i
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/instance: instance-$i
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: postgres
      app.kubernetes.io/instance: instance-$i
  template:
    metadata:
      labels:
        app.kubernetes.io/name: postgres
        app.kubernetes.io/instance: instance-$i
    spec:
      containers:
        - name: postgres
          image: postgres:15-alpine
          ports:
            - containerPort: 5432
              name: postgres
          envFrom:
            - secretRef:
                name: postgres-secret
          volumeMounts:
            - name: postgres-data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            exec:
              command: ["pg_isready", "-U", "whatsapp"]
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "whatsapp"]
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: postgres-data
          persistentVolumeClaim:
            claimName: postgres-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: whatsapp-$i
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/instance: instance-$i
spec:
  type: ClusterIP
  ports:
    - port: 5432
      targetPort: postgres
      name: postgres
  selector:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/instance: instance-$i
EOF

    echo "  ✓ Created instance-$i (phones: $START_PHONE-$END_PHONE)"
done

echo ""
echo "Done! Generated $NUM_INSTANCES instances."
echo ""
echo "To deploy all instances:"
echo "  for i in \$(seq -w 1 $NUM_INSTANCES); do"
echo "    kubectl apply -k overlays/instance-\$i"
echo "  done"
echo ""
echo "To deploy a single instance:"
echo "  kubectl apply -k overlays/instance-01"
