#!/bin/bash
# Deploy all WhatsApp instances to Kubernetes
# Usage: ./deploy-all.sh [action]
# Actions: apply (default), delete, status

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OVERLAYS_DIR="$SCRIPT_DIR/overlays"
ACTION=${1:-apply}

# Find all instance directories
INSTANCES=$(ls -d "$OVERLAYS_DIR"/instance-* 2>/dev/null | sort)

if [ -z "$INSTANCES" ]; then
    echo "No instances found. Run ./generate-instances.sh first."
    exit 1
fi

NUM_INSTANCES=$(echo "$INSTANCES" | wc -l)
echo "Found $NUM_INSTANCES instances"
echo ""

case $ACTION in
    apply)
        echo "Deploying all instances..."
        for instance in $INSTANCES; do
            name=$(basename "$instance")
            echo "  → Deploying $name..."
            kubectl apply -k "$instance"
        done
        echo ""
        echo "✓ All instances deployed!"
        echo ""
        echo "Check status with: $0 status"
        ;;

    delete)
        echo "Deleting all instances..."
        for instance in $INSTANCES; do
            name=$(basename "$instance")
            echo "  → Deleting $name..."
            kubectl delete -k "$instance" --ignore-not-found=true
        done
        echo ""
        echo "✓ All instances deleted!"
        ;;

    status)
        echo "Instance Status:"
        echo "================"
        for instance in $INSTANCES; do
            name=$(basename "$instance")
            namespace="whatsapp-${name#instance-}"

            # Get pod status
            pod_status=$(kubectl get pods -n "$namespace" -l app.kubernetes.io/name=whatsapp-api -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
            db_status=$(kubectl get pods -n "$namespace" -l app.kubernetes.io/name=postgres -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")

            printf "  %-15s API: %-10s DB: %-10s\n" "$name" "$pod_status" "$db_status"
        done
        ;;

    logs)
        instance=${2:-instance-01}
        namespace="whatsapp-${instance#instance-}"
        echo "Streaming logs from $instance..."
        kubectl logs -n "$namespace" -l app.kubernetes.io/name=whatsapp-api -f
        ;;

    *)
        echo "Usage: $0 [apply|delete|status|logs [instance-name]]"
        exit 1
        ;;
esac
