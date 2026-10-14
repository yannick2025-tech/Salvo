#!/usr/bin/env bash
# SO Plugin Management Script
# Wraps `make plugin-*` commands for easier plugin management on server.
#
# Usage:
#   ./scripts/plugin.sh upload <file> <name> <version>   Upload + register + hot-load
#   ./scripts/plugin.sh list                             List all registered plugins
#   ./scripts/plugin.sh delete <id>                      Delete plugin by ID
#   ./scripts/plugin.sh status <id> <enabled|disabled>   Enable/disable plugin
#   ./scripts/plugin.sh upload-all                       Build + upload all plugins
#
# Environment overrides:
#   BACKEND_HOST, BACKEND_PORT, ADMIN_EMAIL, ADMIN_PASSWORD
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# ─── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()   { echo -e "${CYAN}[$(date '+%H:%M:%S')]${NC} $*"; }
ok()    { echo -e "${GREEN}✓${NC} $*"; }
err()   { echo -e "${RED}✗${NC} $*" >&2; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }

# ─── Default plugin list for upload-all ───────────────────────────────────────
ALL_PLUGINS=(aes login paypwd sign)
DEFAULT_VERSION="1.0.0"

usage() {
    cat <<'EOF'
SO Plugin Management

Usage:
  ./scripts/plugin.sh upload <file> <name> <version>
      Upload .so file, register in DB, and hot-load.
      Example: ./scripts/plugin.sh upload plugins/aes.so aes 1.0.0

  ./scripts/plugin.sh upload-all [--version <ver>]
      Build all plugins (make build-all) then upload each one.
      Uses DEFAULT_VERSION or --version override.
      Example: ./scripts/plugin.sh upload-all --version 1.0.1

  ./scripts/plugin.sh list
      List all registered SO plugins.

  ./scripts/plugin.sh delete <id>
      Delete a plugin by its snowflake ID.

  ./scripts/plugin.sh status <id> <enabled|disabled>
      Enable or disable a plugin by ID.

Environment:
  BACKEND_HOST     Backend host (default: from configs/salvo.yaml or 0.0.0.0)
  BACKEND_PORT     Backend port (default: from configs/salvo.yaml or 8766)
  ADMIN_EMAIL      Admin email (default: admin@salvo.local)
  ADMIN_PASSWORD   Admin password (default: admin)
  BUILD_ONLY=1     For upload-all: only build, skip upload (testing builds)
EOF
}

# ─── Helpers ─────────────────────────────────────────────────────────────────
# Get backend URL (host:port), honoring env overrides.
get_backend_url() {
    local host="${BACKEND_HOST:-}"
    local port="${BACKEND_PORT:-}"
    if [[ -z "$host" || -z "$port" ]]; then
        # Try reading from Makefile defaults via `make`
        local info
        info=$(make -s -p 2>/dev/null | grep -E '^(BACKEND_HOST|BACKEND_PORT)' | head -2 || true)
        host=$(echo "$info" | grep BACKEND_HOST | awk -F':= ' '{print $2}' | tr -d ' ' || true)
        port=$(echo "$info" | grep BACKEND_PORT | awk -F':= ' '{print $2}' | tr -d ' ' || true)
    fi
    host="${host:-0.0.0.0}"
    port="${port:-8766}"
    echo "http://${host}:${port}"
}

# Login and print JWT token to stdout.
get_token() {
    local api="$1"
    local email="${ADMIN_EMAIL:-admin@salvo.local}"
    local password="${ADMIN_PASSWORD:-admin}"
    curl -s -X POST "${api}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"${email}\",\"password\":\"${password}\"}" \
        | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))" 2>/dev/null
}

# Upload a single .so file, register, and hot-load.
# Args: <so_file> <plugin_name> <version>
do_upload() {
    local so_file="$1"
    local name="$2"
    local version="$3"

    if [[ ! -f "$so_file" ]]; then
        err "File not found: $so_file"
        return 1
    fi

    local api
    api=$(get_backend_url)
    log "Backend: $api"

    # Step 1: Login
    log "Step 1: Login"
    local token
    token=$(get_token "$api")
    if [[ -z "$token" ]]; then
        err "Login failed (check ADMIN_EMAIL/ADMIN_PASSWORD)"
        return 1
    fi
    ok "Token obtained"

    # Step 2: Upload .so file
    log "Step 2: Upload $so_file"
    local upload_result file_path
    upload_result=$(curl -s -X POST "${api}/api/v1/so-plugins/upload-file" \
        -H "Authorization: Bearer ${token}" \
        -F "file=@${so_file}")
    file_path=$(echo "$upload_result" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('file_path',''))" 2>/dev/null || true)
    if [[ -z "$file_path" ]]; then
        err "Upload failed"
        echo "Response: $upload_result" >&2
        return 1
    fi
    ok "Uploaded to: $file_path"

    # Step 3: Register + Hot-load
    log "Step 3: Register '$name@$version'"
    local create_result create_code
    create_result=$(curl -s -X POST "${api}/api/v1/so-plugins/create" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${token}" \
        -d "{\"name\":\"${name}\",\"version\":\"${version}\",\"file_path\":\"${file_path}\"}")
    create_code=$(echo "$create_result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code', -1))" 2>/dev/null || echo -1)
    if [[ "$create_code" != "0" ]]; then
        local msg
        msg=$(echo "$create_result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('message',''))" 2>/dev/null || echo "unknown")
        err "Registration failed: $msg"
        echo "Response: $create_result" >&2
        return 1
    fi
    ok "Plugin '$name@$version' registered and hot-loaded"
    echo "$create_result" | python3 -m json.tool 2>/dev/null || echo "$create_result"
}

# Build and upload all plugins.
do_upload_all() {
    local version="$DEFAULT_VERSION"
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --version) version="$2"; shift 2 ;;
            *) err "Unknown option: $1"; return 1 ;;
        esac
    done

    log "Building all plugins (make build-all)..."
    if ! make build-all; then
        err "Build failed"
        return 1
    fi
    ok "All plugins built"

    if [[ "${BUILD_ONLY:-0}" == "1" ]]; then
        warn "BUILD_ONLY=1, skipping upload"
        return 0
    fi

    local failed=0
    for name in "${ALL_PLUGINS[@]}"; do
        local so_file="plugins/${name}.so"
        if [[ ! -f "$so_file" ]]; then
            warn "$so_file not found, skipping"
            continue
        fi
        echo ""
        log "=== Uploading: $name ($version) ==="
        if ! do_upload "$so_file" "$name" "$version"; then
            err "Failed: $name"
            ((failed++))
        fi
    done

    echo ""
    if [[ $failed -eq 0 ]]; then
        ok "All plugins uploaded successfully"
    else
        err "$failed plugin(s) failed to upload"
        return 1
    fi
}

# List all registered plugins.
do_list() {
    local api
    api=$(get_backend_url)
    log "Backend: $api"

    local token
    token=$(get_token "$api")
    if [[ -z "$token" ]]; then
        err "Login failed"
        return 1
    fi

    curl -s -X POST "${api}/api/v1/so-plugins/list" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${token}" \
        -d '{"limit":100}' \
        | python3 -m json.tool
}

# Delete a plugin by ID.
do_delete() {
    local id="$1"
    if [[ -z "$id" ]]; then
        err "PLUGIN_ID is required"
        return 1
    fi

    local api
    api=$(get_backend_url)
    log "Backend: $api"

    local token
    token=$(get_token "$api")
    if [[ -z "$token" ]]; then
        err "Login failed"
        return 1
    fi

    log "Deleting plugin ID: $id"
    local result
    result=$(curl -s -X POST "${api}/api/v1/so-plugins/delete" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${token}" \
        -d "{\"id\":${id}}")
    echo "$result" | python3 -m json.tool
    ok "Delete request sent"
}

# Enable/disable a plugin.
do_status() {
    local id="$1"
    local status="$2"
    if [[ -z "$id" ]]; then
        err "PLUGIN_ID is required"
        return 1
    fi
    if [[ -z "$status" ]]; then
        err "PLUGIN_STATUS is required (enabled or disabled)"
        return 1
    fi
    if [[ "$status" != "enabled" && "$status" != "disabled" ]]; then
        err "Invalid status: $status (must be enabled or disabled)"
        return 1
    fi

    local api
    api=$(get_backend_url)
    log "Backend: $api"

    local token
    token=$(get_token "$api")
    if [[ -z "$token" ]]; then
        err "Login failed"
        return 1
    fi

    log "Setting plugin $id to $status"
    local result
    result=$(curl -s -X POST "${api}/api/v1/so-plugins/status" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${token}" \
        -d "{\"id\":${id},\"status\":\"${status}\"}")
    echo "$result" | python3 -m json.tool
    ok "Status updated"
}

# ─── Main ─────────────────────────────────────────────────────────────────────
main() {
    if [[ $# -lt 1 ]]; then
        usage
        exit 1
    fi

    local cmd="$1"
    shift

    case "$cmd" in
        upload)
            if [[ $# -lt 3 ]]; then
                err "Usage: ./scripts/plugin.sh upload <file> <name> <version>"
                exit 1
            fi
            do_upload "$1" "$2" "$3"
            ;;
        upload-all)
            do_upload_all "$@"
            ;;
        list)
            do_list
            ;;
        delete)
            if [[ $# -lt 1 ]]; then
                err "Usage: ./scripts/plugin.sh delete <id>"
                exit 1
            fi
            do_delete "$1"
            ;;
        status)
            if [[ $# -lt 2 ]]; then
                err "Usage: ./scripts/plugin.sh status <id> <enabled|disabled>"
                exit 1
            fi
            do_status "$1" "$2"
            ;;
        -h|--help|help)
            usage
            ;;
        *)
            err "Unknown command: $cmd"
            echo ""
            usage
            exit 1
            ;;
    esac
}

main "$@"
