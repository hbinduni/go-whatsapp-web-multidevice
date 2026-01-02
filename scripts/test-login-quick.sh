#!/bin/bash

# Quick Login Test
# Usage: PASSWORD=xxx ./test-login-quick.sh [phone_number]
#    or: ./test-login-quick.sh [phone_number]  (will prompt for password)

BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-binduni}"
CLIENT_PHONE="${1:-628192191202}"

# Prompt for password if not set
if [ -z "$PASSWORD" ]; then
  read -sp "Enter password for ${USERNAME}: " PASSWORD
  echo ""
fi

echo "=== Quick Login Test ==="
echo "Client: $CLIENT_PHONE"
echo ""

# Get token
echo "[1] Getting JWT token..."
TOKEN=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${USERNAME}\", \"password\": \"${PASSWORD}\"}" \
  | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "ERROR: Failed to get token"
  exit 1
fi
echo "OK - Token received"

# Check status
echo ""
echo "[2] Checking client status..."
curl -s "${BASE_URL}/api/${CLIENT_PHONE}/app/status" \
  -H "Authorization: Bearer ${TOKEN}" | jq .

# Request login
echo ""
echo "[3] Requesting QR login..."
curl -s "${BASE_URL}/api/${CLIENT_PHONE}/app/login" \
  -H "Authorization: Bearer ${TOKEN}" | jq .

echo ""
echo "=== Done ==="
echo ""
echo "To monitor events: curl -N '${BASE_URL}/events' -H 'Authorization: Bearer ${TOKEN}'"
