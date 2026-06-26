.PHONY: all help build-all start restart dev stop \
        clean clean-so clean-db clean-logs \
        test lint \
        plugins-build plugins-clean

CONFIG ?= configs/salvo.yaml
BIN ?= bin/salvo

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

# ============================================================
# Start / Stop / Restart
# ============================================================

start:
	@mkdir -p logs
	@# Start backend
	@lsof -i :8766 >/dev/null 2>&1 && echo "Backend already running on :8766" || ( \
		nohup ./$(BIN) -config $(CONFIG) > logs/salvo-stdout.log 2> logs/salvo-stderr.log & \
		echo "Backend started (PID $$!)" \
	)
	@# Start frontend
	@lsof -i :3000 >/dev/null 2>&1 && echo "Frontend already running on :3000" || ( \
		cd web/app && nohup npm run dev > ../../logs/frontend.log 2>&1 & \
		echo "Frontend started" \
	)
	@sleep 2
	@echo ""
	@echo "  → Frontend: http://localhost:3000"
	@echo "  → Backend:  http://localhost:8766"

stop:
	@-pkill -f "bin/salvo" 2>/dev/null || true
	@-pkill -f "vite" 2>/dev/null || true
	@-pkill -f "node.*vite" 2>/dev/null || true
	@sleep 1
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

lint:
	go vet ./...

# ============================================================
# SO Plugin targets (aliases)
# ============================================================

plugins-build: build-all

plugins-clean: clean-so
