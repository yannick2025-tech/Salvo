.PHONY: all backend frontend dev dev-backend dev-frontend build clean test lint help plugins-build plugins-clean

CONFIG ?= configs/salvo.yaml
BIN ?= bin/salvo

all: build

help:
	@echo "Salvo - HTTP Performance Testing Tool"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build backend binary"
	@echo "  backend        Build and run backend"
	@echo "  frontend       Install deps and run frontend dev server"
	@echo "  dev            Run both backend and frontend (dev mode)"
	@echo "  dev-backend    Run backend in dev mode (hot reload)"
	@echo "  dev-frontend   Run frontend dev server only"
	@echo "  build-frontend Build frontend for production"
	@echo "  clean          Remove build artifacts and temp files"
	@echo "  test           Run all Go tests"
	@echo "  lint           Run Go linter"
	@echo "  stop           Stop running backend process"
	@echo "  restart        Stop and restart backend"
	@echo "  plugins-build  Build all SO plugins under plugins/"
	@echo "  plugins-clean  Remove all compiled .so files under plugins/"

build:
	go build -o $(BIN) ./cmd/salvo

backend: build
	mkdir -p logs
	./$(BIN) -config $(CONFIG)

dev-backend:
	mkdir -p logs
	SALVO_ROOT=$(shell pwd) go run ./cmd/salvo -config $(CONFIG)

dev-frontend:
	@find web/app/src -name "*.vue.js" -delete
	cd web/app && npm install && npm run dev

dev:
	@echo "Starting backend and frontend (dev mode)..."
	@echo "  → Frontend: http://localhost:3000  (Vite HMR, use this port!)"
	@echo "  → Backend:  http://localhost:8766"
	@find web/app/src -name "*.vue.js" -delete
	@cd web/app && npm install
	@make dev-backend & make dev-frontend & wait

build-frontend:
	cd web/app && npm install && npm run build

clean:
	rm -rf bin/ logs/ *.db *.db-shm *.db-wal salvo
	rm -rf web/app/dist web/app/node_modules web/app/.vite web/dist
	find web/app/src -name "*.vue.js" -delete
	@echo "Cleaned build artifacts and temp files"

test:
	go test -v -count=1 ./...

lint:
	go vet ./...

stop:
	-pkill -f "bin/salvo" 2>/dev/null || true
	-pkill -f "salvo" 2>/dev/null || true
	-pkill -f "go run ./cmd/salvo" 2>/dev/null || true
	-pkill -f "vite" 2>/dev/null || true
	-pkill -f "node.*vite" 2>/dev/null || true
	@sleep 1 && echo "stopped"

restart: stop backend

# --- SO Plugin targets ---

# Auto-discover plugin directories (each must contain a main.go)
PLUGIN_DIRS := $(shell find plugins -mindepth 1 -maxdepth 1 -type d)

plugins-build:
	@echo "Building SO plugins..."
	@count=0; \
	for dir in $(PLUGIN_DIRS); do \
		name=$$(basename $$dir); \
		echo "  → building $$name..."; \
		go build -buildmode=plugin -o $$dir/$$name.so $$dir/main.go || exit 1; \
		count=$$((count + 1)); \
	done; \
	echo "Done. Built $$count plugin(s)."

plugins-clean:
	@echo "Cleaning SO plugins..."
	@find plugins -name '*.so' -delete
	@echo "Done."
