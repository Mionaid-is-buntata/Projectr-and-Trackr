.PHONY: build run dev test test-race lint tidy arm deploy

-include .env
export

# Deploy target defaults — override in .env or via shell env
DEPLOY_USER ?= your-user
DEPLOY_HOST ?= your-host-or-ip
DEPLOY_DIR  ?= /opt/projctr

# Build for ARM64 (run this on the Raspberry Pi)
build:
	go build -o projctr ./cmd/server

# Run the built binary
run: build
	./projctr

# Live reload during development (requires Air: go install github.com/air-verse/air@latest)
# Uses testdata/ as the local Huntr jobs source; override with HUNTR_JOBS_PATH if needed.
dev:
	air

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Tidy module dependencies
tidy:
	go mod tidy

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# ── Deployment targets (run these from the Fedora workstation) ──────────────

# Cross-compile for Raspberry Pi ARM64
arm:
	GOOS=linux GOARCH=arm64 go build -o projctr-arm64 ./cmd/server

# Copy binary and assets to the deploy target and restart the service
deploy: arm
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "mkdir -p $(DEPLOY_DIR)/config $(DEPLOY_DIR)/docs"
	rsync -avz projctr-arm64 infrastructure/projctr.service $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_DIR)/
	rsync -avz config/ $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_DIR)/config/
	rsync -avz --delete docs/ $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_DIR)/docs/
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "cp $(DEPLOY_DIR)/projctr $(DEPLOY_DIR)/projctr.prev 2>/dev/null || true"
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "mv $(DEPLOY_DIR)/projctr-arm64 $(DEPLOY_DIR)/projctr && chmod +x $(DEPLOY_DIR)/projctr"
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "sudo cp $(DEPLOY_DIR)/projctr.service /etc/systemd/system/projctr.service 2>/dev/null || true; sudo systemctl daemon-reload && sudo systemctl restart projctr"
