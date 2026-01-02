#!/bin/bash

# Test Client Management Script for WhatsApp Multi-Client API
# This script tests adding and removing WhatsApp clients dynamically

set -e

# Configuration - modify these as needed
BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${USERNAME:-binduni}"

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
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper function to make authenticated requests
auth_request() {
  local method="$1"
  local endpoint="$2"
  local data="$3"

  if [ -n "$data" ]; then
    curl -s -X "$method" "${BASE_URL}${endpoint}" \
      -H "Authorization: Bearer ${ACCESS_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "$data"
  else
    curl -s -X "$method" "${BASE_URL}${endpoint}" \
      -H "Authorization: Bearer ${ACCESS_TOKEN}"
  fi
}

# Helper function to print section header
print_header() {
  echo ""
  echo -e "${BLUE}========================================${NC}"
  echo -e "${BLUE}  $1${NC}"
  echo -e "${BLUE}========================================${NC}"
  echo ""
}

# Helper function to print step
print_step() {
  echo -e "${CYAN}[$1]${NC} $2"
}

# Helper function to print success
print_success() {
  echo -e "${GREEN}  ✓ $1${NC}"
}

# Helper function to print error
print_error() {
  echo -e "${RED}  ✗ $1${NC}"
}

# Helper function to print warning
print_warning() {
  echo -e "${YELLOW}  ! $1${NC}"
}

print_header "WhatsApp Client Management Test"

echo -e "Base URL: ${YELLOW}${BASE_URL}${NC}"
echo -e "Username: ${YELLOW}${USERNAME}${NC}"
echo ""

# Step 1: Authenticate
print_step "1" "Authenticating..."
AUTH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"${USERNAME}\", \"password\": \"${PASSWORD}\"}")

if echo "$AUTH_RESPONSE" | grep -q "access_token"; then
  ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
  print_success "Authentication successful"
else
  print_error "Authentication failed"
  echo "$AUTH_RESPONSE" | jq . 2>/dev/null || echo "$AUTH_RESPONSE"
  exit 1
fi

# Step 2: List current clients
print_step "2" "Listing current clients..."
CLIENTS_RESPONSE=$(auth_request GET "/admin/clients")
echo "$CLIENTS_RESPONSE" | jq . 2>/dev/null || echo "$CLIENTS_RESPONSE"

CLIENT_COUNT=$(echo "$CLIENTS_RESPONSE" | jq -r '.data.client_count // 0' 2>/dev/null || echo "0")
print_success "Current client count: $CLIENT_COUNT"

echo ""

# Menu for operations
while true; do
  print_header "Client Management Menu"
  echo "1) Add a new client"
  echo "2) Remove a client"
  echo "3) List all clients"
  echo "4) Get client status"
  echo "5) Connect a client"
  echo "6) Disconnect a client"
  echo "7) Exit"
  echo ""
  read -p "Select operation [1-7]: " choice

  case $choice in
    1)
      # Add Client
      print_header "Add New Client"
      read -p "Enter phone number (e.g., 628123456789): " NEW_PHONE
      read -p "Enter display name (optional): " DISPLAY_NAME

      if [ -z "$NEW_PHONE" ]; then
        print_error "Phone number is required"
        continue
      fi

      print_step "ADD" "Adding client $NEW_PHONE..."

      if [ -n "$DISPLAY_NAME" ]; then
        ADD_DATA="{\"phone\": \"${NEW_PHONE}\", \"display_name\": \"${DISPLAY_NAME}\"}"
      else
        ADD_DATA="{\"phone\": \"${NEW_PHONE}\"}"
      fi

      ADD_RESPONSE=$(auth_request POST "/admin/clients" "$ADD_DATA")
      echo "$ADD_RESPONSE" | jq . 2>/dev/null || echo "$ADD_RESPONSE"

      STATUS=$(echo "$ADD_RESPONSE" | jq -r '.data.status // "unknown"' 2>/dev/null)
      MESSAGE=$(echo "$ADD_RESPONSE" | jq -r '.data.message // ""' 2>/dev/null)

      if [ "$STATUS" = "awaiting_login" ]; then
        print_success "Client added successfully"
        echo -e "${YELLOW}  Next step: $MESSAGE${NC}"
      elif [ "$STATUS" = "already_registered" ]; then
        print_warning "Client is already registered"
      else
        # Check for error
        ERROR=$(echo "$ADD_RESPONSE" | jq -r '.message // ""' 2>/dev/null)
        if [ -n "$ERROR" ] && [ "$ERROR" != "null" ]; then
          print_error "$ERROR"
        fi
      fi
      ;;

    2)
      # Remove Client
      print_header "Remove Client"

      # First show current clients
      echo "Current clients:"
      auth_request GET "/admin/clients" | jq -r '.data.clients | to_entries[] | "  - \(.key): \(.value.is_connected) (connected), \(.value.is_logged_in) (logged in)"' 2>/dev/null || echo "  (no clients)"
      echo ""

      read -p "Enter phone number to remove: " REMOVE_PHONE

      if [ -z "$REMOVE_PHONE" ]; then
        print_error "Phone number is required"
        continue
      fi

      read -p "Are you sure you want to remove $REMOVE_PHONE? (y/N): " CONFIRM
      if [[ ! $CONFIRM =~ ^[Yy]$ ]]; then
        print_warning "Cancelled"
        continue
      fi

      print_step "REMOVE" "Removing client $REMOVE_PHONE..."

      REMOVE_RESPONSE=$(auth_request DELETE "/admin/clients/${REMOVE_PHONE}")
      echo "$REMOVE_RESPONSE" | jq . 2>/dev/null || echo "$REMOVE_RESPONSE"

      CODE=$(echo "$REMOVE_RESPONSE" | jq -r '.code // 0' 2>/dev/null)
      if [ "$CODE" = "200" ]; then
        print_success "Client removed successfully"
        echo -e "${YELLOW}  Note: Chat history has been preserved${NC}"
      else
        ERROR=$(echo "$REMOVE_RESPONSE" | jq -r '.message // "Unknown error"' 2>/dev/null)
        print_error "$ERROR"
      fi
      ;;

    3)
      # List clients
      print_header "All Registered Clients"
      CLIENTS_RESPONSE=$(auth_request GET "/admin/clients")
      echo "$CLIENTS_RESPONSE" | jq . 2>/dev/null || echo "$CLIENTS_RESPONSE"
      ;;

    4)
      # Get client status
      print_header "Get Client Status"
      read -p "Enter phone number: " STATUS_PHONE

      if [ -z "$STATUS_PHONE" ]; then
        print_error "Phone number is required"
        continue
      fi

      print_step "STATUS" "Getting status for $STATUS_PHONE..."
      STATUS_RESPONSE=$(auth_request GET "/admin/clients/${STATUS_PHONE}/status")
      echo "$STATUS_RESPONSE" | jq . 2>/dev/null || echo "$STATUS_RESPONSE"
      ;;

    5)
      # Connect client
      print_header "Connect Client"
      read -p "Enter phone number: " CONNECT_PHONE

      if [ -z "$CONNECT_PHONE" ]; then
        print_error "Phone number is required"
        continue
      fi

      print_step "CONNECT" "Connecting client $CONNECT_PHONE..."
      CONNECT_RESPONSE=$(auth_request POST "/admin/clients/${CONNECT_PHONE}/connect")
      echo "$CONNECT_RESPONSE" | jq . 2>/dev/null || echo "$CONNECT_RESPONSE"
      ;;

    6)
      # Disconnect client
      print_header "Disconnect Client"
      read -p "Enter phone number: " DISCONNECT_PHONE

      if [ -z "$DISCONNECT_PHONE" ]; then
        print_error "Phone number is required"
        continue
      fi

      print_step "DISCONNECT" "Disconnecting client $DISCONNECT_PHONE..."
      DISCONNECT_RESPONSE=$(auth_request POST "/admin/clients/${DISCONNECT_PHONE}/disconnect")
      echo "$DISCONNECT_RESPONSE" | jq . 2>/dev/null || echo "$DISCONNECT_RESPONSE"
      ;;

    7)
      print_header "Goodbye!"
      exit 0
      ;;

    *)
      print_error "Invalid option. Please select 1-7."
      ;;
  esac
done
