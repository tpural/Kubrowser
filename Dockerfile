# Multi-stage build for both frontend and backend

# ============================================
# Frontend Build Stage
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Install pnpm
RUN npm install -g pnpm@10

# Copy workspace config and lockfile
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json ./frontend/

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy source code
COPY frontend/ ./frontend/

# Build Next.js application
RUN pnpm --filter frontend build

# ============================================
# Backend Build Stage
# ============================================
FROM golang:1.22-alpine AS backend-builder

WORKDIR /build

# Copy go mod files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy all backend source code
COPY backend/cmd ./cmd
COPY backend/internal ./internal

# Build backend binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# ============================================
# Final Production Stage
# ============================================
FROM node:20-alpine

# Install ca-certificates and pnpm
RUN apk --no-cache add ca-certificates \
    && npm install -g pnpm@10

WORKDIR /app

# Copy workspace config
COPY package.json pnpm-workspace.yaml ./

# Copy frontend build from frontend-builder
COPY --from=frontend-builder /app/frontend/.next ./frontend/.next
COPY --from=frontend-builder /app/frontend/public ./frontend/public
COPY --from=frontend-builder /app/frontend/package.json ./frontend/
COPY --from=frontend-builder /app/frontend/next.config.ts ./frontend/
COPY --from=frontend-builder /app/node_modules ./node_modules
COPY --from=frontend-builder /app/frontend/node_modules ./frontend/node_modules

# Copy backend binary from backend-builder
COPY --from=backend-builder /build/server ./backend/server

# Create startup script
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'set -e' >> /app/start.sh && \
    echo 'cd /app && ./backend/server &' >> /app/start.sh && \
    echo 'BACKEND_PID=$!' >> /app/start.sh && \
    echo 'cd /app/frontend && pnpm start &' >> /app/start.sh && \
    echo 'FRONTEND_PID=$!' >> /app/start.sh && \
    echo 'cleanup() { kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; wait; exit 0; }' >> /app/start.sh && \
    echo 'trap cleanup SIGTERM SIGINT' >> /app/start.sh && \
    echo 'wait' >> /app/start.sh && \
    chmod +x /app/start.sh

EXPOSE 3000 8080

ENV NODE_ENV=production
ENV PORT=8080
ENV NEXT_PUBLIC_API_URL=http://localhost:8080

CMD ["/app/start.sh"]
