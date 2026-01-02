#!/bin/bash

# Test Login Script for WhatsApp Multi-Client API
# This script tests the login flow including phone validation

set -e

# Configuration - modify these as needed
BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-binduni}"
CLIENT_PHONE="${CLIENT_PHONE:-628192191202}"

# Prompt for password if not set
if [ -z "$PASSWORD" ]; then
  read -sp "Enter password for ${USERNAME}: " PASSWORD
  echo ""
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  WhatsApp Login Test Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "Base URL: ${YELLOW}${BASE_URL}${NC}"
echo -e "Username: ${YELLOW}${USERNAME}${NC}"
echo -e "Client Phone: ${YELLOW}${CLIENT_PHONE}${NC}"
echo ""

# Step 1: Login to get JWT token
echo -e "${BLUE}[1/5] Authenticating...${NC}"
AUTH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${USERNAME}\", \"password\": \"${PASSWORD}\"}")

# Check if login was successful
if echo "$AUTH_RESPONSE" | grep -q "access_token"; then
  ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
  echo -e "${GREEN}✓ Authentication successful${NC}"
  echo -e "  Token: ${ACCESS_TOKEN:0:50}..."
else
  echo -e "${RED}✗ Authentication failed${NC}"
  echo "$AUTH_RESPONSE" | jq . 2>/dev/null || echo "$AUTH_RESPONSE"
  exit 1
fi

echo ""

# Step 2: Check client status
echo -e "${BLUE}[2/5] Checking client status...${NC}"
STATUS_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/${CLIENT_PHONE}/app/status" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}")

echo "$STATUS_RESPONSE" | jq . 2>/dev/null || echo "$STATUS_RESPONSE"

IS_LOGGED_IN=$(echo "$STATUS_RESPONSE" | grep -o '"is_logged_in":[^,}]*' | cut -d':' -f2)
echo ""

if [ "$IS_LOGGED_IN" = "true" ]; then
  echo -e "${GREEN}✓ Client is already logged in${NC}"
  echo ""

  # Step 3: Get devices
  echo -e "${BLUE}[3/5] Getting linked devices...${NC}"
  DEVICES_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/${CLIENT_PHONE}/app/devices" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
  echo "$DEVICES_RESPONSE" | jq . 2>/dev/null || echo "$DEVICES_RESPONSE"

else
  echo -e "${YELLOW}! Client is not logged in${NC}"
  echo ""

  # Step 3: Request QR code login
  echo -e "${BLUE}[3/5] Requesting QR code for login...${NC}"
  LOGIN_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/${CLIENT_PHONE}/app/login" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "$LOGIN_RESPONSE" | jq . 2>/dev/null || echo "$LOGIN_RESPONSE"

  QR_LINK=$(echo "$LOGIN_RESPONSE" | grep -o '"qr_link":"[^"]*"' | cut -d'"' -f4)

  if [ -n "$QR_LINK" ]; then
    echo ""
    echo -e "${GREEN}✓ QR code generated${NC}"
    echo -e "  QR Link: ${YELLOW}${QR_LINK}${NC}"
    echo ""
    echo -e "${YELLOW}Please scan the QR code with WhatsApp on phone: ${CLIENT_PHONE}${NC}"
    echo -e "${RED}WARNING: If you scan with a different phone number, login will be rejected!${NC}"
  fi
fi

echo ""

# Step 4: List all clients (admin endpoint)
echo -e "${BLUE}[4/5] Listing all registered clients...${NC}"
CLIENTS_RESPONSE=$(curl -s -X GET "${BASE_URL}/admin/clients" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}")
echo "$CLIENTS_RESPONSE" | jq . 2>/dev/null || echo "$CLIENTS_RESPONSE"

echo ""

# Step 5: Monitor SSE events (optional - runs for 30 seconds)
echo -e "${BLUE}[5/5] Monitor SSE events? (for login status updates)${NC}"
read -p "Press 'y' to monitor events for 30 seconds, any other key to skip: " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo -e "${YELLOW}Monitoring SSE events for 30 seconds...${NC}"
  echo -e "${YELLOW}Look for LOGIN_SUCCESS or LOGIN_REJECTED events${NC}"
  echo ""

  # Use timeout to limit SSE monitoring
  timeout 30 curl -s -N "${BASE_URL}/events" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Accept: text/event-stream" | while read -r line; do
    if [[ "$line" == data:* ]]; then
      echo -e "${GREEN}Event:${NC} $line"

      # Highlight important events
      if echo "$line" | grep -q "LOGIN_REJECTED"; then
        echo -e "${RED}>>> LOGIN REJECTED - Phone mismatch detected! <<<${NC}"
      elif echo "$line" | grep -q "LOGIN_SUCCESS"; then
        echo -e "${GREEN}>>> LOGIN SUCCESS <<<${NC}"
      elif echo "$line" | grep -q "LOGIN_REJECTED_COMPLETE"; then
        echo -e "${RED}>>> MISMATCHED DEVICE DISCONNECTED <<<${NC}"
      fi
    fi
  done || true

  echo ""
  echo -e "${YELLOW}SSE monitoring ended${NC}"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Test Complete${NC}"
echo -e "${BLUE}========================================${NC}"
