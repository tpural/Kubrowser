# Kubrowser

A browser-based terminal tool that allows you to access kubectl through temporary Kubernetes pods. Each browser session spawns a dedicated pod with kubectl installed, providing isolated terminal access via WebSocket.

## Features

- 🌐 **Browser-based terminal** - Access kubectl from any modern web browser
- 🚀 **Automatic pod management** - Temporary pods are created and cleaned up automatically
- 🔒 **Session isolation** - Each user session gets its own isolated pod
- ⚡ **Real-time terminal** - WebSocket-based terminal with full TTY support

## Prerequisites

- Go 1.22 or later
- Node.js 20.9.0 or later
- Kubernetes cluster access
- kubectl configured (for local development)
- Docker (for containerized deployment)

## Quick Start

### Local Development

1. **Install dependencies**

   ```bash
   # Backend
   cd backend && go mod download

   # Frontend
   cd frontend && npm install
   ```

2. **Start development servers**

   ```bash
   make dev
   ```

   This starts both the backend (port 8080) and frontend (port 3000).

3. **Access the application**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080

### Setup Kubernetes RBAC

Before using Kubrowser, you need to set up the required RBAC permissions:

```bash
# Backend RBAC (for creating/deleting pods)
kubectl apply -f k8s/rbac.yaml

# Kubectl pod RBAC (required for pods to have kubectl permissions)
kubectl apply -f k8s/kubectl-pod-rbac.yaml
```

## Project Structure

```
Kubrowser/
├── backend/          # Go backend API server
│   ├── cmd/server/   # Main application entry point
│   └── internal/     # Internal packages
│       ├── api/      # HTTP/WebSocket handlers
│       ├── k8s/      # Kubernetes client and pod management
│       ├── session/  # Session management
│       └── terminal/ # Terminal execution
├── frontend/         # Next.js frontend
│   ├── app/          # Next.js app directory
│   └── components/   # React components
├── k8s/              # Kubernetes manifests
└── scripts/          # Utility scripts
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
