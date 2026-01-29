.PHONY: help install lint lint-fix format format-check test build clean dev backend-lint frontend-lint frontend-format

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dependencies for backend and frontend
	@echo "Installing backend dependencies..."
	cd backend && go mod download && go mod tidy
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Installing pre-commit hooks..."
	@command -v pre-commit >/dev/null 2>&1 || { echo "pre-commit not found. Install with: pip3 install --user pre-commit"; exit 1; }
	pre-commit install

lint: backend-lint frontend-lint ## Run all linters

lint-fix: backend-lint-fix frontend-lint-fix ## Run all linters with auto-fix

backend-lint: ## Run Go linter (golangci-lint)
	@echo "Running golangci-lint..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/"; exit 1; }
	cd backend && golangci-lint run --timeout=5m

backend-lint-fix: ## Run Go linter with auto-fix
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/"; exit 1; }
	cd backend && golangci-lint run --fix --timeout=5m

frontend-lint: ## Run ESLint for frontend
	@echo "Running ESLint..."
	cd frontend && npm run lint

frontend-lint-fix: ## Run ESLint with auto-fix
	cd frontend && npm run lint:fix

format: frontend-format backend-format ## Format all code

format-check: frontend-format-check backend-format-check ## Check code formatting

frontend-format: ## Format frontend code with Prettier
	cd frontend && npm run format

frontend-format-check: ## Check frontend code formatting
	cd frontend && npm run format:check

backend-format: ## Format Go code
	cd backend && gofmt -w . && goimports -w -local github.com/kubrowser .

backend-format-check: ## Check Go code formatting
	cd backend && gofmt -d . && goimports -d -local github.com/kubrowser .

test: ## Run all tests
	@echo "Running backend tests..."
	cd backend && go test ./...
	@echo "Running frontend type check..."
	cd frontend && npm run type-check

build: ## Build backend and frontend
	@echo "Building backend..."
	cd backend && go build -o bin/server ./cmd/server
	@echo "Building frontend..."
	cd frontend && npm run build

clean: ## Clean build artifacts
	rm -rf backend/bin
	rm -rf frontend/.next
	rm -rf frontend/out
	rm -rf frontend/node_modules/.cache

dev: ## Start development servers
	@echo "Starting development servers..."
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:3000"
	@if [ -f .env ]; then \
		echo "Loading environment from .env file..."; \
	fi
	@export KUBECONFIG_PATH=$${HOME}/.kube/config; \
	export NVM_DIR="$$HOME/.nvm"; \
	[ -s "$$NVM_DIR/nvm.sh" ] && \. "$$NVM_DIR/nvm.sh"; \
	[ -s "$$NVM_DIR/nvm.sh" ] && nvm use 20; \
	trap 'kill 0' EXIT; \
	if [ -f .env ]; then \
		set -a; \
		. ./.env; \
		set +a; \
	fi; \
	cd backend && KUBECONFIG_PATH=$${HOME}/.kube/config go run ./cmd/server & \
	cd frontend && npm run dev & \
	wait

pre-commit-install: ## Install pre-commit hooks
	@command -v pre-commit >/dev/null 2>&1 || { echo "pre-commit not found. Install with: pip3 install --user pre-commit"; exit 1; }
	pre-commit install

pre-commit-run: ## Run pre-commit on all files
	@command -v pre-commit >/dev/null 2>&1 || { echo "pre-commit not found. Install with: pip3 install --user pre-commit"; exit 1; }
	pre-commit run --all-files

kill-sessions: ## Kill all kubrowser sessions
	@./scripts/kill-sessions.sh --all

kill-session: ## Kill a specific kubrowser session (usage: make kill-session SESSION=<pod-name>)
	@if [ -z "$(SESSION)" ]; then \
		echo "Usage: make kill-session SESSION=<pod-name-or-session-id>"; \
		exit 1; \
	fi
	@./scripts/kill-session.sh $(SESSION)
