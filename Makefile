.PHONY: all help build-all rebuild start restart dev stop \
        clean clean-so clean-db clean-logs \
        test lint \
        plugins-build plugins-clean \
        plugin-upload plugin-list plugin-delete plugin-status

CONFIG ?= configs/salvo.yaml
BIN ?= bin/salvo

# Use bash so disown is available (sh on macOS lacks reliable job-control escape)
SHELL := /bin/bash

# Auto-discover plugin directories (each must contain a main.go)
PLUGIN_DIRS := $(shell find plugins -mindepth 1 -maxdepth 1 -type d ! -name shared)

# Extract a scalar value from a top-level YAML section in $(CONFIG).
# Usage: $(call yaml_get,<section>,<key>)
# NOTE: awk body intentionally avoids sub()/gsub() calls so that no stray ")"
# leaks into Make's $(shell ...) parser.
yaml_get = $(shell awk '/^$(1):/{f=1;next} /^[A-Za-z]/{f=0} f&&/^[[:space:]]+$(2):/{print $$2; exit}' $(CONFIG) | tr -d '"')

BACKEND_HOST  := $(or $(call yaml_get,server,host),0.0.0.0)
BACKEND_PORT  := $(or $(call yaml_get,server,port),8766)
FRONTEND_HOST := $(or $(call yaml_get,frontend,host),localhost)
FRONTEND_PORT := $(or $(call yaml_get,frontend,port),3000)

all: build-all

help:
	@echo "Salvo - HTTP Performance Testing Tool"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run:"
	@echo "  build-all      Compile SO plugins + main binary in one session (version-safe)"
	@echo "  rebuild        clean-so + build-all (rebuild plugins & binary, keep db & logs)"
	@echo "  start          Start backend + frontend in background (no compile)"
	@echo "  restart        Stop + start backend + frontend (no compile)"
	@echo "  dev            build-all + start (one-shot compile and launch)"
	@echo "  stop           Stop all running processes"
	@echo ""
	@echo "Clean:"
	@echo "  clean-so       Remove compiled .so files only"
	@echo "  clean-db       Remove database files only"
	@echo "  clean-logs     Remove log files only"
	@echo "  clean          Remove all artifacts (logs, db, so, bin, frontend dist)"
	@echo ""
	@echo "Other:"
	@echo "  test              Run all Go tests"
	@echo "  lint              Run Go linter"
	@echo "  assets            Fetch go:embed assets (echarts.min.js) required for build"
	@echo "  frontend-deps     Install frontend npm dependencies (web/app/node_modules)"
	@echo "  plugins-build     Build all SO plugins under plugins/"
	@echo "  plugins-clean     Remove all compiled .so files (alias for clean-so)"
	@echo ""
	@echo "SO Plugin Management (remote):"
	@echo "  plugin-upload     Upload .so file, register in DB, and hot-load (no restart)"
	@echo "                    Usage: make plugin-upload PLUGIN_FILE=plugins/xxx.so PLUGIN_NAME=xxx PLUGIN_VERSION=1.0.0"
	@echo "  plugin-list       List all registered SO plugins"
	@echo "  plugin-delete     Delete a plugin by ID (usage: PLUGIN_ID=<id>)"
	@echo "  plugin-status     Enable/disable a plugin (usage: PLUGIN_ID=<id> PLUGIN_STATUS=enabled|disabled)"

# ============================================================
# Build
# ============================================================

# frontend-deps: install npm dependencies for the Vite frontend.
# Skips when web/app/node_modules/.bin/vite already present (fast no-op in CI
# or re-runs).  This target is a prerequisite of `assets` (so it runs on
# `make build-all`) and is also checked before `make start` launches the
# frontend dev server — ensuring a fresh clone / new environment never
# hits the "sh: 1: vite not found" error.
.PHONY: frontend-deps
frontend-deps:
	@if [ ! -x web/app/node_modules/.bin/vite ]; then \
		echo "Installing frontend dependencies (web/app/node_modules missing)..."; \
		cd web/app && npm install || { echo "ERROR: npm install failed" >&2; exit 1; }; \
	else \
		echo "Frontend dependencies OK (vite found in node_modules)."; \
	fi

# assets: ensure embedded assets required by go:embed exist.
# - echarts.min.js is sourced from frontend node_modules if available,
#   otherwise fetched from CDN matching the version pinned in
#   web/app/package.json (so a fresh clone without npm install can still build).
assets: frontend-deps
	@mkdir -p internal/api
	@if [ -f internal/api/echarts.min.js ]; then \
		echo "echarts.min.js already present, skip."; \
	elif [ -f web/app/node_modules/echarts/dist/echarts.min.js ]; then \
		cp web/app/node_modules/echarts/dist/echarts.min.js internal/api/echarts.min.js; \
		echo "Copied echarts.min.js from web/app/node_modules."; \
	else \
		echo "Fetching echarts.min.js from CDN (web/app/node_modules not available)..."; \
		echarts_ver=$$(node -p "require('./web/app/package.json').dependencies.echarts" 2>/dev/null | sed 's/[\^~]//'); \
		[ -z "$$echarts_ver" ] && echarts_ver="5"; \
		echo "  using echarts@$$echarts_ver"; \
		curl -fsSL "https://cdn.jsdelivr.net/npm/echarts@$$echarts_ver/dist/echarts.min.js" \
			-o internal/api/echarts.min.js || { \
				echo "ERROR: failed to fetch echarts.min.js, run 'cd web/app && npm install' first" >&2; exit 1; \
			}; \
		echo "Fetched echarts.min.js from CDN."; \
	fi

# Compile plugins and main binary in the same build session
# to ensure identical build cache (required by Go plugin mechanism).
build-all: assets
	@echo "Building SO plugins + main binary..."
	@for dir in $(PLUGIN_DIRS); do \
		name=$$(basename $$dir); \
		echo "  → building $$name.so..."; \
		go build -buildmode=plugin -o plugins/$$name.so $$dir/main.go || exit 1; \
	done
	go build -o $(BIN) ./cmd/salvo
	@echo "Build complete."

# Rebuild plugins + binary from scratch (clean stale .so first to avoid
# Go plugin version mismatch), keeping database and logs intact.
# Use this after code changes that don't require a data reset.
rebuild: clean-so build-all
	@echo "Rebuild complete (db & logs preserved)."

# ============================================================
# Start / Stop / Restart
# ============================================================

start: frontend-deps
	@mkdir -p logs
	@# Start backend
	@if lsof -i :$(BACKEND_PORT) >/dev/null 2>&1; then \
		echo "Backend already running on :$(BACKEND_PORT)"; \
	else \
		nohup ./$(BIN) -config $(CONFIG) >> logs/salvo.log 2>&1 & \
		echo "Backend started (PID $$!)"; \
		disown $$! 2>/dev/null || true; \
	fi
	@# Start frontend (disown so it survives the make shell exit; avoids SIGHUP killing vite cold start)
	@# Verify with HTTP probe, not just lsof, to avoid being fooled by a dying process that still holds the port.
	@if curl -s -o /dev/null --max-time 1 http://$(FRONTEND_HOST):$(FRONTEND_PORT)/ >/dev/null 2>&1; then \
		echo "Frontend already running on :$(FRONTEND_PORT)"; \
	else \
		if lsof -i :$(FRONTEND_PORT) >/dev/null 2>&1; then \
			echo "Port :$(FRONTEND_PORT) held by a non-responsive process, cleaning up..."; \
			pkill -9 -f "node.*vite" 2>/dev/null || true; \
			pkill -9 -f "vite" 2>/dev/null || true; \
			sleep 1; \
		fi; \
		cd web/app && SALVO_BACKEND_PORT=$(BACKEND_PORT) SALVO_FRONTEND_PORT=$(FRONTEND_PORT) SALVO_FRONTEND_HOST=$(FRONTEND_HOST) nohup npm run dev > ../../logs/frontend.log 2>&1 & \
		echo "Frontend started (PID $$!)"; \
		disown $$! 2>/dev/null || true; \
	fi
	@# Wait for ports to be ready (vite cold start after clean needs more time)
	@echo "Waiting for services to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		b=$$(lsof -i :$(BACKEND_PORT) >/dev/null 2>&1 && echo y || echo n); \
		f=$$(curl -s -o /dev/null --max-time 1 http://$(FRONTEND_HOST):$(FRONTEND_PORT)/ >/dev/null 2>&1 && echo y || echo n); \
		if [ "$$b" = "y" ] && [ "$$f" = "y" ]; then \
			echo "  Both services ready"; break; \
		fi; \
		[ $$i -eq 15 ] && echo "  WARNING: services not ready after 15s (backend=$$b frontend=$$f)"; \
		sleep 1; \
	done
	@echo ""
	@echo "  → Frontend: http://$(FRONTEND_HOST):$(FRONTEND_PORT)"
	@echo "  → Backend:  http://$(BACKEND_HOST):$(BACKEND_PORT)"

stop:
	@# Send SIGTERM first to allow graceful shutdown
	@-pkill -TERM -f "bin/salvo" 2>/dev/null || true
	@-pkill -TERM -f "node.*vite" 2>/dev/null || true
	@-pkill -TERM -f "npm run dev" 2>/dev/null || true
	@# Wait up to 5s for processes to exit, then force kill any survivors
	@for i in 1 2 3 4 5; do \
		pgrep -f "bin/salvo" >/dev/null 2>&1 || pgrep -f "node.*vite" >/dev/null 2>&1 || break; \
		sleep 1; \
	done
	@-pkill -KILL -f "bin/salvo" 2>/dev/null || true
	@-pkill -KILL -f "node.*vite" 2>/dev/null || true
	@# Confirm ports are released
	@for i in 1 2 3 4 5; do \
		lsof -i :$(BACKEND_PORT) >/dev/null 2>&1 || lsof -i :$(FRONTEND_PORT) >/dev/null 2>&1 || break; \
		sleep 1; \
	done
	@echo "All processes stopped."

restart: stop start

# ============================================================
# Dev (compile + start)
# ============================================================

dev: build-all start

# ============================================================
# Clean
# ============================================================

clean-so:
	@find plugins -name '*.so' -delete
	@echo "Removed .so files."

clean-db:
	@rm -f *.db *.db-shm *.db-wal
	@echo "Removed database files."

clean-logs:
	@rm -rf logs/
	@mkdir -p logs
	@echo "Removed log files."

clean: clean-logs clean-db clean-so
	@rm -rf bin/ salvo
	@rm -rf web/app/dist web/app/.vite web/dist web/app/node_modules/.vite
	@find web/app/src -name "*.js" -delete 2>/dev/null || true
	@echo "Cleaned all artifacts."

# ============================================================
# Test & Lint
# ============================================================

test:
	go test -v -count=1 ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

cover-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	go vet ./...

# ============================================================
# SO Plugin targets (aliases)
# ============================================================

plugins-build: build-all

plugins-clean: clean-so

# ============================================================
# SO Plugin Management (remote API)
# ============================================================

# Default admin credentials (override via env)
ADMIN_EMAIL   ?= admin@salvo.local
ADMIN_PASSWORD ?= admin

# Helper: login and print JWT token to stdout.
# Usage: $(call get_token)
define get_token
$(shell curl -s -X POST http://$(BACKEND_HOST):$(BACKEND_PORT)/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"$(ADMIN_EMAIL)","password":"$(ADMIN_PASSWORD)"}' \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))" 2>/dev/null)
endef

# plugin-upload — upload .so file → register in DB → hot-load in one shot.
#
# Required variables:
#   PLUGIN_FILE    — path to the .so file (e.g. plugins/myplugin.so)
#   PLUGIN_NAME    — plugin name  (e.g. myplugin)
#   PLUGIN_VERSION — plugin version (e.g. 1.0.0)
#
# The backend host/port are read from configs/salvo.yaml by default (BACKEND_HOST / BACKEND_PORT).
plugin-upload:
	@set -e; \
	if [ -z "$(PLUGIN_FILE)" ]; then echo "ERROR: PLUGIN_FILE is required (e.g. PLUGIN_FILE=plugins/myplugin.so)"; exit 1; fi; \
	if [ -z "$(PLUGIN_NAME)" ]; then echo "ERROR: PLUGIN_NAME is required (e.g. PLUGIN_NAME=myplugin)"; exit 1; fi; \
	if [ -z "$(PLUGIN_VERSION)" ]; then echo "ERROR: PLUGIN_VERSION is required (e.g. PLUGIN_VERSION=1.0.0)"; exit 1; fi; \
	if [ ! -f "$(PLUGIN_FILE)" ]; then echo "ERROR: file not found: $(PLUGIN_FILE)"; exit 1; fi; \
	API="http://$(BACKEND_HOST):$(BACKEND_PORT)"; \
	echo "Backend: $$API"; \
	echo ""; \
	echo "=== Step 1: Login ==="; \
	TOKEN=$$(curl -s -X POST "$$API/api/v1/auth/login" \
		-H "Content-Type: application/json" \
		-d '{"email":"$(ADMIN_EMAIL)","password":"$(ADMIN_PASSWORD)"}' \
		| python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))"); \
	if [ -z "$$TOKEN" ]; then echo "ERROR: login failed (check ADMIN_EMAIL/ADMIN_PASSWORD)"; exit 1; fi; \
	echo "  ✓ Token obtained"; \
	echo ""; \
	echo "=== Step 2: Upload .so file ==="; \
	UPLOAD_RESULT=$$(curl -s -X POST "$$API/api/v1/so-plugins/upload-file" \
		-H "Authorization: Bearer $$TOKEN" \
		-F "file=@$(PLUGIN_FILE)"); \
	FILE_PATH=$$(echo "$$UPLOAD_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('file_path',''))"); \
	if [ -z "$$FILE_PATH" ]; then echo "ERROR: file upload failed"; echo "Response: $$UPLOAD_RESULT"; exit 1; fi; \
	echo "  ✓ Uploaded to: $$FILE_PATH"; \
	echo ""; \
	echo "=== Step 3: Register in DB + Hot-load ==="; \
	CREATE_RESULT=$$(curl -s -X POST "$$API/api/v1/so-plugins/create" \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d "{\"name\":\"$(PLUGIN_NAME)\",\"version\":\"$(PLUGIN_VERSION)\",\"file_path\":\"$$FILE_PATH\"}"); \
	CREATE_CODE=$$(echo "$$CREATE_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('code', -1))"); \
	if [ "$$CREATE_CODE" != "0" ]; then \
		MSG=$$(echo "$$CREATE_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('message',''))"); \
		echo "ERROR: registration failed: $$MSG"; \
		echo "Response: $$CREATE_RESULT"; \
		exit 1; \
	fi; \
	echo "  ✓ Plugin '$(PLUGIN_NAME)@$(PLUGIN_VERSION)' registered and hot-loaded"; \
	echo ""; \
	echo "=== Done ==="; \
	echo "$$CREATE_RESULT" | python3 -m json.tool

# plugin-list — list all registered SO plugins.
plugin-list:
	@set -e; \
	API="http://$(BACKEND_HOST):$(BACKEND_PORT)"; \
	TOKEN=$$(curl -s -X POST "$$API/api/v1/auth/login" \
		-H "Content-Type: application/json" \
		-d '{"email":"$(ADMIN_EMAIL)","password":"$(ADMIN_PASSWORD)"}' \
		| python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))"); \
	if [ -z "$$TOKEN" ]; then echo "ERROR: login failed"; exit 1; fi; \
	curl -s -X POST "$$API/api/v1/so-plugins/list" \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d '{"limit":100}' \
		| python3 -m json.tool

# plugin-delete — delete a plugin by ID (removes DB record + .so file).
# Usage: make plugin-delete PLUGIN_ID=<snowflake-id>
plugin-delete:
	@set -e; \
	if [ -z "$(PLUGIN_ID)" ]; then echo "ERROR: PLUGIN_ID is required (e.g. PLUGIN_ID=332367573066723328)"; exit 1; fi; \
	API="http://$(BACKEND_HOST):$(BACKEND_PORT)"; \
	TOKEN=$$(curl -s -X POST "$$API/api/v1/auth/login" \
		-H "Content-Type: application/json" \
		-d '{"email":"$(ADMIN_EMAIL)","password":"$(ADMIN_PASSWORD)"}' \
		| python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))"); \
	if [ -z "$$TOKEN" ]; then echo "ERROR: login failed"; exit 1; fi; \
	RESULT=$$(curl -s -X POST "$$API/api/v1/so-plugins/delete" \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d "{\"id\":$(PLUGIN_ID)}"); \
	echo "$$RESULT" | python3 -m json.tool

# plugin-status — enable or disable a plugin.
# Usage: make plugin-status PLUGIN_ID=<id> PLUGIN_STATUS=enabled|disabled
plugin-status:
	@set -e; \
	if [ -z "$(PLUGIN_ID)" ]; then echo "ERROR: PLUGIN_ID is required"; exit 1; fi; \
	if [ -z "$(PLUGIN_STATUS)" ]; then echo "ERROR: PLUGIN_STATUS is required (enabled or disabled)"; exit 1; fi; \
	API="http://$(BACKEND_HOST):$(BACKEND_PORT)"; \
	TOKEN=$$(curl -s -X POST "$$API/api/v1/auth/login" \
		-H "Content-Type: application/json" \
		-d '{"email":"$(ADMIN_EMAIL)","password":"$(ADMIN_PASSWORD)"}' \
		| python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))"); \
	if [ -z "$$TOKEN" ]; then echo "ERROR: login failed"; exit 1; fi; \
	RESULT=$$(curl -s -X POST "$$API/api/v1/so-plugins/status" \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $$TOKEN" \
		-d "{\"id\":$(PLUGIN_ID),\"status\":\"$(PLUGIN_STATUS)\"}"); \
	echo "$$RESULT" | python3 -m json.tool
