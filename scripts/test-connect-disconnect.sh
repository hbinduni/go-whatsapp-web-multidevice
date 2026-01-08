#!/bin/bash

# Test Script for Connect/Disconnect Client APIs
# Usage: ./test-connect-disconnect.sh [connect|disconnect|status|list|cycle] <phone_number>
#
# Examples:
#   PASSWORD=admin123 ./test-connect-disconnect.sh list
#   PASSWORD=admin123 ./test-connect-disconnect.sh status 628123456789
#   PASSWORD=admin123 ./test-connect-disconnect.sh connect 628123456789
#   PASSWORD=admin123 ./test-connect-disconnect.sh disconnect 628123456789
#   PASSWORD=admin123 ./test-connect-disconnect.sh cycle 628123456789  # disconnect then connect

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
CYAN='\033[0;36m'
NC='\033[0m'

# Parse arguments
ACTION="${1:-list}"
PHONE="${2:-}"

usage() {
  echo "Usage: $0 [connect|disconnect|status|list|cycle] [phone_number]"
  echo ""
  echo "Actions:"
  echo "  list       - List all clients with their connection status"
  echo "  status     - Get status of a specific client (requires phone)"
  echo "  connect    - Connect a client (requires phone)"
  echo "  disconnect - Disconnect a client (requires phone)"
  echo "  cycle      - Disconnect then connect (for testing, requires phone)"
  echo ""
  echo "Environment variables:"
  echo "  BASE_URL  - API base URL (default: http://localhost:8080)"
  echo "  USERNAME  - Auth username (default: binduni)"
  echo "  PASSWORD  - Auth password (required)"
  echo ""
  echo "Examples:"
  echo "  PASSWORD=secret $0 list"
  echo "  PASSWORD=secret $0 status 628123456789"
  echo "  PASSWORD=secret $0 connect 628123456789"
  echo "  PASSWORD=secret $0 disconnect 628123456789"
  echo "  PASSWORD=secret $0 cycle 628123456789"
  exit 1
}

# Check password
if [ -z "$PASSWORD" ]; then
  echo -e "${RED}Error: PASSWORD environment variable is required${NC}"
  usage
fi

# Check phone for actions that require it
if [ "$ACTION" != "list" ] && [ -z "$PHONE" ]; then
  echo -e "${RED}Error: Phone number required for action '$ACTION'${NC}"
  usage
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Connect/Disconnect Client Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Base URL: ${YELLOW}${BASE_URL}${NC}"
echo -e "Action:   ${YELLOW}${ACTION}${NC}"
if [ -n "$PHONE" ]; then
  echo -e "Phone:    ${YELLOW}${PHONE}${NC}"
fi
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

# Function to list clients
list_clients() {
  echo ""
  echo -e "${BLUE}[LIST]${NC} Listing all clients..."

  RESPONSE=$(curl -s -X GET "${BASE_URL}/admin/clients" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  # Pretty print with status indicators
  echo ""
  echo "$RESPONSE" | jq -r '.results.clients // .data.clients // {} | to_entries[] |
    "  \(.key): " +
    (if .value.is_logged_in then "🟢 Logged In"
     elif .value.is_connected then "🟡 Connected"
     else "⚪ Disconnected" end) +
    " (status: \(.value.status))"' 2>/dev/null || echo "$RESPONSE" | jq .

  echo ""
  COUNT=$(echo "$RESPONSE" | jq -r '.results.client_count // .data.client_count // 0' 2>/dev/null)
  echo -e "${GREEN}  Total clients: $COUNT${NC}"
}

# Function to get client status
get_status() {
  local phone="$1"

  echo ""
  echo -e "${BLUE}[STATUS]${NC} Getting status for: $phone"

  RESPONSE=$(curl -s -X GET "${BASE_URL}/admin/clients/${phone}/status" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

  IS_CONNECTED=$(echo "$RESPONSE" | jq -r '.results.is_connected // .data.is_connected // false' 2>/dev/null)
  IS_LOGGED_IN=$(echo "$RESPONSE" | jq -r '.results.is_logged_in // .data.is_logged_in // false' 2>/dev/null)

  echo ""
  if [ "$IS_LOGGED_IN" = "true" ]; then
    echo -e "  Status: ${GREEN}🟢 Logged In${NC}"
  elif [ "$IS_CONNECTED" = "true" ]; then
    echo -e "  Status: ${YELLOW}🟡 Connected (not logged in)${NC}"
  else
    echo -e "  Status: ${CYAN}⚪ Disconnected${NC}"
  fi
}

# Function to connect client
connect_client() {
  local phone="$1"

  echo ""
  echo -e "${BLUE}[CONNECT]${NC} Connecting client: $phone"

  # Get status before
  echo -e "${CYAN}  Before:${NC}"
  BEFORE=$(curl -s -X GET "${BASE_URL}/admin/clients/${phone}/status" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
  BEFORE_CONNECTED=$(echo "$BEFORE" | jq -r '.results.is_connected // .data.is_connected // false' 2>/dev/null)
  BEFORE_LOGGED_IN=$(echo "$BEFORE" | jq -r '.results.is_logged_in // .data.is_logged_in // false' 2>/dev/null)
  echo -e "    is_connected: $BEFORE_CONNECTED, is_logged_in: $BEFORE_LOGGED_IN"

  # Connect
  echo -e "${CYAN}  Calling connect...${NC}"
  RESPONSE=$(curl -s -X POST "${BASE_URL}/admin/clients/${phone}/connect" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "  Response: $(echo "$RESPONSE" | jq -c . 2>/dev/null || echo "$RESPONSE")"

  # Get status after (wait a moment for connection to establish)
  sleep 1
  echo -e "${CYAN}  After:${NC}"
  AFTER=$(curl -s -X GET "${BASE_URL}/admin/clients/${phone}/status" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
  AFTER_CONNECTED=$(echo "$AFTER" | jq -r '.results.is_connected // .data.is_connected // false' 2>/dev/null)
  AFTER_LOGGED_IN=$(echo "$AFTER" | jq -r '.results.is_logged_in // .data.is_logged_in // false' 2>/dev/null)
  echo -e "    is_connected: $AFTER_CONNECTED, is_logged_in: $AFTER_LOGGED_IN"

  # Check if state changed
  echo ""
  if [ "$BEFORE_CONNECTED" = "$AFTER_CONNECTED" ]; then
    if [ "$AFTER_CONNECTED" = "true" ]; then
      echo -e "${YELLOW}  ⚠ Client was already connected${NC}"
    else
      echo -e "${RED}  ✗ Connect had no effect (still disconnected)${NC}"
    fi
  else
    if [ "$AFTER_CONNECTED" = "true" ]; then
      echo -e "${GREEN}  ✓ Client connected successfully${NC}"
    else
      echo -e "${RED}  ✗ Connect failed (state went from connected to disconnected?)${NC}"
    fi
  fi
}

# Function to disconnect client
disconnect_client() {
  local phone="$1"

  echo ""
  echo -e "${BLUE}[DISCONNECT]${NC} Disconnecting client: $phone"

  # Get status before
  echo -e "${CYAN}  Before:${NC}"
  BEFORE=$(curl -s -X GET "${BASE_URL}/admin/clients/${phone}/status" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
  BEFORE_CONNECTED=$(echo "$BEFORE" | jq -r '.results.is_connected // .data.is_connected // false' 2>/dev/null)
  BEFORE_LOGGED_IN=$(echo "$BEFORE" | jq -r '.results.is_logged_in // .data.is_logged_in // false' 2>/dev/null)
  echo -e "    is_connected: $BEFORE_CONNECTED, is_logged_in: $BEFORE_LOGGED_IN"

  # Disconnect
  echo -e "${CYAN}  Calling disconnect...${NC}"
  RESPONSE=$(curl -s -X POST "${BASE_URL}/admin/clients/${phone}/disconnect" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

  echo "  Response: $(echo "$RESPONSE" | jq -c . 2>/dev/null || echo "$RESPONSE")"

  # Get status after
  sleep 1
  echo -e "${CYAN}  After:${NC}"
  AFTER=$(curl -s -X GET "${BASE_URL}/admin/clients/${phone}/status" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
  AFTER_CONNECTED=$(echo "$AFTER" | jq -r '.results.is_connected // .data.is_connected // false' 2>/dev/null)
  AFTER_LOGGED_IN=$(echo "$AFTER" | jq -r '.results.is_logged_in // .data.is_logged_in // false' 2>/dev/null)
  echo -e "    is_connected: $AFTER_CONNECTED, is_logged_in: $AFTER_LOGGED_IN"

  # Check if state changed
  echo ""
  if [ "$BEFORE_CONNECTED" = "$AFTER_CONNECTED" ]; then
    if [ "$AFTER_CONNECTED" = "false" ]; then
      echo -e "${YELLOW}  ⚠ Client was already disconnected${NC}"
    else
      echo -e "${RED}  ✗ Disconnect had no effect (still connected)${NC}"
    fi
  else
    if [ "$AFTER_CONNECTED" = "false" ]; then
      echo -e "${GREEN}  ✓ Client disconnected successfully${NC}"
    else
      echo -e "${RED}  ✗ Disconnect failed (state went from disconnected to connected?)${NC}"
    fi
  fi
}

# Execute based on action
case $ACTION in
  list)
    list_clients
    ;;
  status)
    get_status "$PHONE"
    ;;
  connect)
    connect_client "$PHONE"
    ;;
  disconnect)
    disconnect_client "$PHONE"
    ;;
  cycle)
    echo -e "${YELLOW}Running disconnect + connect test cycle...${NC}"
    disconnect_client "$PHONE"
    echo ""
    echo -e "${YELLOW}Waiting 2 seconds before reconnecting...${NC}"
    sleep 2
    connect_client "$PHONE"
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
