.PHONY: all help build-all rebuild start restart dev stop \
        clean clean-so clean-db clean-logs \
        test lint \
        plugins-build plugins-clean

CONFIG ?= configs/salvo.yaml
BIN ?= bin/salvo

# Use bash so disown is available (sh on macOS lacks reliable job-control escape)
SHELL := /bin/bash

# Auto-discover plugin directories (each must contain a main.go)
PLUGIN_DIRS := $(shell find plugins -mindepth 1 -maxdepth 1 -type d)

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
	@echo "  test           Run all Go tests"
	@echo "  lint           Run Go linter"
	@echo "  plugins-build  Build all SO plugins under plugins/"
	@echo "  plugins-clean  Remove all compiled .so files (alias for clean-so)"

# ============================================================
# Build
# ============================================================

# Compile plugins and main binary in the same build session
# to ensure identical build cache (required by Go plugin mechanism).
build-all:
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

start:
	@mkdir -p logs
	@# Start backend
	@if lsof -i :8766 >/dev/null 2>&1; then \
		echo "Backend already running on :8766"; \
	else \
		nohup ./$(BIN) -config $(CONFIG) > logs/salvo-stdout.log 2> logs/salvo-stderr.log & \
		echo "Backend started (PID $$!)"; \
		disown $$! 2>/dev/null || true; \
	fi
	@# Start frontend (disown so it survives the make shell exit; avoids SIGHUP killing vite cold start)
	@# Verify with HTTP probe, not just lsof, to avoid being fooled by a dying process that still holds the port.
	@if curl -s -o /dev/null --max-time 1 http://localhost:3000/ >/dev/null 2>&1; then \
		echo "Frontend already running on :3000"; \
	else \
		if lsof -i :3000 >/dev/null 2>&1; then \
			echo "Port :3000 held by a non-responsive process, cleaning up..."; \
			pkill -9 -f "node.*vite" 2>/dev/null || true; \
			pkill -9 -f "vite" 2>/dev/null || true; \
			sleep 1; \
		fi; \
		cd web/app && nohup npm run dev > ../../logs/frontend.log 2>&1 & \
		echo "Frontend started (PID $$!)"; \
		disown $$! 2>/dev/null || true; \
	fi
	@# Wait for ports to be ready (vite cold start after clean needs more time)
	@echo "Waiting for services to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		b=$$(lsof -i :8766 >/dev/null 2>&1 && echo y || echo n); \
		f=$$(curl -s -o /dev/null --max-time 1 http://localhost:3000/ >/dev/null 2>&1 && echo y || echo n); \
		if [ "$$b" = "y" ] && [ "$$f" = "y" ]; then \
			echo "  Both services ready"; break; \
		fi; \
		[ $$i -eq 15 ] && echo "  WARNING: services not ready after 15s (backend=$$b frontend=$$f)"; \
		sleep 1; \
	done
	@echo ""
	@echo "  → Frontend: http://localhost:3000"
	@echo "  → Backend:  http://localhost:8766"

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
		lsof -i :8766 >/dev/null 2>&1 || lsof -i :3000 >/dev/null 2>&1 || break; \
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
	@rm -rf web/app/dist web/app/.vite web/dist
	@find web/app/src -name "*.vue.js" -delete 2>/dev/null || true
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
