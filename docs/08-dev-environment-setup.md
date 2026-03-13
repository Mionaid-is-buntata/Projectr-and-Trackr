# Development Environment Setup

**Version:** 1.0  
**Date:** 19 February 2026

---

## Overview

The network has the following machines relevant to Projctr:

| Host | Role |
|------|------|
| `your-workstation.local` | Fedora development workstation — editor, cross-compilation, Git pushes to Gitea |
| `your-pi.local` | Raspberry Pi 5 — production deployment target; runs Huntr, Qdrant, and Projctr |
| `your-llm-host.local` | Lower-power host — capable of running a lightweight LLM for optional extraction assistance |
| `your-workstation.local` | Fedora workstation — also available as the high-power LLM host when the LLM host's resources are insufficient |

Source code is version-controlled in a **Gitea instance running on the local network**. The Go module path and remote origin URL should point at that Gitea instance. No code is pushed to GitHub or any external service.

Development happens on the workstation with VS Code or Cursor connecting to the Raspberry Pi via SSH. SSH key pairs are already in place. Qdrant is already running on the Raspberry Pi.

The intended workflow is to write and run code directly on the Raspberry Pi via the remote SSH session in the editor, using the Raspberry Pi itself as the development environment. This avoids any cross-compilation overhead during active development and means Projctr always runs against the real Huntr database and the real Qdrant instance. Cross-compilation from the workstation is used only to produce clean deployment builds.

---

## 1. Workstation Prerequisites (the workstation — Fedora)

### 1.1 Go

Install Go 1.21 or later. 1.21 is the minimum — it introduces `log/slog`, used throughout for structured logging.

```bash
# Check what's available in Fedora repos
dnf info golang

# Install from repos if version is >= 1.21
sudo dnf install golang

# If the repo version is too old, install manually
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

Verify:

```bash
go version
# go version go1.23.x linux/amd64
```

### 1.2 Cross-Compilation Target (ARM64)

Go's cross-compilation support is built in — no extra toolchain is needed. Test it works on the workstation:

```bash
GOOS=linux GOARCH=arm64 go build -o /tmp/test-arm64 .
file /tmp/test-arm64
# /tmp/test-arm64: ELF 64-bit LSB executable, ARM aarch64
```

### 1.3 SQLite CLI

Useful for inspecting `projctr.db` on the Raspberry Pi or running the Huntr discovery queries locally if you copy the database over.

```bash
sudo dnf install sqlite
```

---

## 2. Pi 5 (the Raspberry Pi) Setup

### 2.1 Go on the Raspberry Pi

Go must also be installed on the Raspberry Pi since code runs there during development. Check if it is already present:

```bash
ssh $DEPLOY_USER@your-pi.local "go version"
```

If not installed:

```bash
ssh $DEPLOY_USER@your-pi.local

# On the Raspberry Pi — ARM64 build of Go
wget https://go.dev/dl/go1.23.0.linux-arm64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### 2.2 Create the Project Directory

```bash
ssh $DEPLOY_USER@your-pi.local "mkdir -p ~/projctr"
```

### 2.3 Verify Qdrant Is Running

```bash
ssh $DEPLOY_USER@your-pi.local "curl -s http://localhost:6333/ | python3 -m json.tool"
# Should return Qdrant version info

# List existing collections
ssh $DEPLOY_USER@your-pi.local "curl -s http://localhost:6333/collections | python3 -m json.tool"
```

If Qdrant is not responding, check the service:

```bash
ssh $DEPLOY_USER@your-pi.local "sudo systemctl status qdrant"
```

---

## 3. Editor Setup

### 3.1 VS Code Remote SSH

VS Code's Remote - SSH extension connects directly to the Raspberry Pi. With SSH keys already in place:

1. Open the Command Palette → **Remote-SSH: Connect to Host**
2. Enter `$DEPLOY_USER@your-pi.local`
3. VS Code installs its server component on the Raspberry Pi automatically
4. Open `~/projctr` as the workspace folder

Install the **Go** extension (by Go Team at Google) inside the remote session — it installs gopls, delve, and other tooling directly on the Raspberry Pi.

### 3.2 Cursor

Cursor's SSH remote works identically to VS Code's. Connect via the remote pane and open `~/projctr`. Install the Go language support extension in the remote context.

### 3.3 Go Tools on the Raspberry Pi

Once connected remotely, install Go language tooling on the Raspberry Pi (VS Code/Cursor will prompt for this automatically):

```bash
# gopls — language server
go install golang.org/x/tools/gopls@latest

# delve — debugger
go install github.com/go-delve/delve/cmd/dlv@latest

# goimports — import formatting
go install golang.org/x/tools/cmd/goimports@latest

# Air — live reload during development
go install github.com/air-verse/air@latest
```

---

## 4. Project Initialisation (on the Raspberry Pi)

All of the following commands run on the Raspberry Pi, either in the remote terminal or in the editor's integrated terminal.

### 4.1 Module Setup

```bash
cd ~/projctr
git init
go mod init gitea.local/yourname/projctr    # Use your Gitea instance hostname and username
git remote add origin http://gitea.local/yourname/projctr.git
```

Create the repository in Gitea first (via the web UI), then add the remote as above. The Gitea hostname may differ — check what is used on the local network.

### 4.2 Core Dependencies

```bash
# HTTP router
go get github.com/go-chi/chi/v5

# SQLite — pure Go, no CGo required
go get modernc.org/sqlite

# Qdrant Go client
go get github.com/qdrant/go-client

# TOML config parsing
go get github.com/BurntSushi/toml
```

### 4.3 Directory Structure

```bash
mkdir -p cmd/server
mkdir -p internal/{config,database,vectordb,huntr,ingestion,extraction,clustering,gap,briefs,pipeline,handlers,models}
mkdir -p templates/{layouts,pages,partials}
mkdir -p static
touch config.toml Makefile README.md
```

---

## 5. Configuration

Create `config.toml` on the Raspberry Pi. Since development runs directly on the Raspberry Pi, paths point at the real Huntr database. Use a separate Qdrant collection name for development to avoid polluting the production collection while the schema is being settled.

```toml
[server]
port = 8090
host = "0.0.0.0"

[database]
path = "./projctr.db"

[huntr]
db_path = "/home/your-user/huntr/data/huntr.db"    # Confirm actual path via discovery runbook
score_threshold = 300

[qdrant]
host               = "localhost"
port               = 6334
collection         = "projctr_dev"           # Use projctr_pain_points only in production
huntr_cv_collection = ""                     # Fill in after discovery
vector_dimensions  = 0                       # Fill in after discovery

[embedding]
model    = ""                                # Fill in after discovery
endpoint = ""                                # Fill in after discovery
# Endpoint will be on the LLM host or the workstation depending on model size — see §LLM Hosts below

[extraction]
mode = "rules"
tech_dictionary = "./tech-dict.toml"

[extraction.llm]
enabled  = false
endpoint = "http://your-llm-host.local:11434"      # Default to the LLM host; switch to the workstation if needed
model    = ""                                # Fill in after discovery / selection

[clustering]
min_cluster_size    = 3
similarity_threshold = 0.65

[ingestion]
schedule = ""                                # Disabled during development
```

Add to `.gitignore`:

```
projctr.db
projctr-dev.db
config.local.toml
```

---

## 6. Makefile

The Makefile lives on the Raspberry Pi and handles the local build/run cycle. The cross-compile target is run from the Fedora workstation when producing a clean deployment binary.

```makefile
.PHONY: build run dev test lint tidy

# Build for the Raspberry Pi's native ARM64
build:
	go build -o projctr ./cmd/server

# Run
run: build
	./projctr

# Live reload during development (requires Air)
dev:
	air

# Tests
test:
	go test ./...

test-race:
	go test -race ./...

# Tidy modules
tidy:
	go mod tidy

# Lint (install golangci-lint first — see below)
lint:
	golangci-lint run ./...
```

For the **cross-compile and deploy** workflow from the workstation:

```makefile
# On the workstation — cross-compile and deploy
arm:
	GOOS=linux GOARCH=arm64 go build -o projctr-arm64 ./cmd/server

deploy: arm
	rsync -avz --exclude='.git' \
	  projctr-arm64 templates/ static/ config.toml \
	  $DEPLOY_USER@your-pi.local:~/projctr/
	ssh $DEPLOY_USER@your-pi.local "mv ~/projctr/projctr-arm64 ~/projctr/projctr && chmod +x ~/projctr/projctr"
	ssh $DEPLOY_USER@your-pi.local "sudo systemctl restart projctr"
```

---

## 7. Live Reload with Air

Air watches for source file changes and restarts the server automatically. Useful when working on templates or handlers.

Create `.air.toml` on the Raspberry Pi:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/projctr ./cmd/server"
bin = "./tmp/projctr"
include_ext = ["go", "html", "css"]
exclude_dir = ["tmp", "vendor"]

[log]
time = false

[misc]
clean_on_exit = true
```

Run with:

```bash
air
```

---

## 8. Linting (Optional)

```bash
# Install golangci-lint on the Raspberry Pi
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
  sh -s -- -b $(go env GOPATH)/bin latest

golangci-lint run ./...
```

---

## 9. LLM Hosts

Projctr has two uses for a local LLM: optional pain point extraction assistance and, separately, text embedding for clustering and gap analysis. Both are served via Ollama on one of two local hosts depending on the resource requirements of the model in use.

| Host | Typical use |
|------|-------------|
| `your-llm-host.local` | Default LLM host — use for lightweight models (embedding, small extraction models). Lower power consumption, always available. |
| `your-workstation.local` | Fallback LLM host — use when a larger model is needed for higher-quality extraction. More capable but is the active dev machine, so availability depends on workload. |

The decision of which host to use is made per model, not per feature. The config keys are separate so they can point at different hosts simultaneously:

```toml
[embedding]
endpoint = "http://your-llm-host.local:11434/api/embeddings"    # LLM host for embedding (lightweight)

[extraction.llm]
endpoint = "http://your-llm-host.local:11434/api/generate"      # Start on the LLM host; move to the workstation if quality is poor
```

**When to escalate from the LLM host to the workstation:** if extraction quality from the LLM-host-hosted model is visibly poor (pain points are vague, technologies are missed, domain context is wrong), switch the `extraction.llm.endpoint` to `http://your-workstation.local:11434/...` and try a larger model. Embedding does not need this — a lightweight model is sufficient for semantic similarity.

**Scaffolding note:** project scaffolding (directory structure, initial files, boilerplate) is determined on a per-project basis and is not automated by Projctr. The architecture document's project structure (`§6`) describes Projctr's own layout; for portfolio projects generated from briefs, scaffolding is a manual step informed by the brief's recommended technology stack.

---

## 10. Verifying the Setup

Run through this checklist once the project structure is in place:

```bash
# On the Raspberry Pi (via remote terminal in editor)

# 1. Go is installed and correct version
go version

# 2. Qdrant is reachable
curl -s http://localhost:6333/ | python3 -m json.tool

# 3. Huntr SQLite is readable (path from discovery runbook)
sqlite3 /path/to/huntr.db ".tables"

# 4. Module compiles cleanly
go build ./...

# 5. Tests pass (once written)
go test ./...
```

When all five pass, the environment is ready for Phase 1 implementation.

---

## 11. Typical Development Loop

1. Open VS Code or Cursor, connect to `$DEPLOY_USER@your-pi.local`
2. Open `~/projctr` as the workspace
3. Run `air` in the integrated terminal — server starts with live reload
4. Edit Go source, handlers, or templates; Air restarts automatically
5. Test via browser at `http://your-pi.local:8090` from any device on the local network
6. Commit and push to Gitea when a phase milestone is complete
7. When deploying a clean build: run `make arm && make deploy` from the workstation
