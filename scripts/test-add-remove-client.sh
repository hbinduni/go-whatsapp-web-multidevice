#!/bin/bash

# Quick Test Script for Add/Remove Client APIs
# Usage: ./test-add-remove-client.sh [add|remove|both] <phone_number>
#
# Examples:
#   ./test-add-remove-client.sh add 628123456789
#   ./test-add-remove-client.sh remove 628123456789
#   ./test-add-remove-client.sh both 628999999999  # adds then removes

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-binduni}"
PASSWORD="${PASSWORD:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse arguments
ACTION="${1:-both}"
PHONE="${2:-628999999999}"
DISPLAY_NAME="${3:-Test Client}"

usage() {
  echo "Usage: $0 [add|remove|both|list] [phone_number] [display_name]"
  echo ""
  echo "Actions:"
  echo "  add     - Add a new client"
  echo "  remove  - Remove an existing client"
  echo "  both    - Add then remove (for testing)"
  echo "  list    - List all clients"
  echo ""
  echo "Environment variables:"
  echo "  BASE_URL  - API base URL (default: http://localhost:8080)"
  echo "  USERNAME  - Auth username (default: binduni)"
  echo "  PASSWORD  - Auth password (required)"
  echo ""
  echo "Examples:"
  echo "  PASSWORD=secret $0 add 628123456789"
  echo "  PASSWORD=secret $0 remove 628123456789"
  echo "  PASSWORD=secret $0 both 628999999999 'Test Client'"
  exit 1
}

# Check password
if [ -z "$PASSWORD" ]; then
  echo -e "${RED}Error: PASSWORD environment variable is required${NC}"
  usage
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Add/Remove Client Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Base URL: ${YELLOW}${BASE_URL}${NC}"
echo -e "Action:   ${YELLOW}${ACTION}${NC}"
echo -e "Phone:    ${YELLOW}${PHONE}${NC}"
echo ""

# Authenticate
echo -e "${BLUE}[AUTH]${NC} Authenticating..."
AUTH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${USERNAME}\", \"password\": \"${PASSWORD}\"}")

if echo "$AUTH_RESPONSE" | grep -q "access_token"; then
  ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
  echo -e "${GREEN}  ✓ Authenticated${NC}"
else
  echo -e "${RED}  ✗ Authentication failed${NC}"
  echo "$AUTH_RESPONSE" | jq . 2>/dev/null || echo "$AUTH_RESPONSE"
  exit 1
fi

# Function to add client
add_client() {
  local phone="$1"
  local name="$2"

  echo ""
  echo -e "${BLUE}[ADD]${NC} Adding client: $phone"

  RESPONSE=$(curl -s -X POST "${BASE_URL}/admin/clients" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"phone\": \"${phone}\", \"display_name\": \"${name}\"}")

  echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

  STATUS=$(echo "$RESPONSE" | jq -r '.results.status // .data.status // "error"' 2>/dev/null)
  if [ "$STATUS" = "awaiting_login" ] || [ "$STATUS" = "already_registered" ]; then
    echo -e "${GREEN}  ✓ Add client: $STATUS${NC}"
    return 0
  else
    echo -e "${RED}  ✗ Add client failed${NC}"
    return 1
  fi
}

# Function to remove client
remove_client() {
  local phone="$1"

  echo ""
  echo -e "${BLUE}[REMOVE]${NC} Removing client: $phone"

  RESPONSE=$(curl -s -X DELETE "${BASE_URL}/admin/clients/${phone}" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

  CODE=$(echo "$RESPONSE" | jq -r '.code // ""' 2>/dev/null)
  if [ "$CODE" = "200" ] || [ "$CODE" = "SUCCESS" ]; then
    echo -e "${GREEN}  ✓ Remove client successful${NC}"
    return 0
  else
    echo -e "${RED}  ✗ Remove client failed${NC}"
    return 1
  fi
}

# Function to list clients
list_clients() {
  echo ""
  echo -e "${BLUE}[LIST]${NC} Listing all clients..."

  RESPONSE=$(curl -s -X GET "${BASE_URL}/admin/clients" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

  COUNT=$(echo "$RESPONSE" | jq -r '.results.client_count // .data.client_count // 0' 2>/dev/null)
  echo -e "${GREEN}  ✓ Total clients: $COUNT${NC}"
}

# Execute based on action
case $ACTION in
  add)
    add_client "$PHONE" "$DISPLAY_NAME"
    ;;
  remove)
    remove_client "$PHONE"
    ;;
  both)
    echo -e "${YELLOW}Running add + remove test cycle...${NC}"
    add_client "$PHONE" "$DISPLAY_NAME"
    sleep 2
    remove_client "$PHONE"
    ;;
  list)
    list_clients
    ;;
  *)
    echo -e "${RED}Unknown action: $ACTION${NC}"
    usage
    ;;
esac

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}  Test Complete${NC}"
echo -e "${BLUE}========================================${NC}"
