# Multi-stage build for both frontend and backend

# ============================================
# Frontend Build Stage
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY frontend/ ./

# Build Next.js application
RUN npm run build

# ============================================
# Backend Build Stage
# ============================================
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app/backend

# Copy go mod files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source code
COPY backend/ ./

# Build backend binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# ============================================
# Final Production Stage
# ============================================
FROM node:20-alpine

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy frontend build from frontend-builder
COPY --from=frontend-builder /app/frontend/.next ./frontend/.next
COPY --from=frontend-builder /app/frontend/public ./frontend/public
COPY --from=frontend-builder /app/frontend/package*.json ./frontend/
COPY --from=frontend-builder /app/frontend/next.config.ts ./frontend/
COPY --from=frontend-builder /app/frontend/tsconfig.json ./frontend/

# Install only production dependencies for Next.js
WORKDIR /app/frontend
RUN npm ci --only=production

# Copy backend binary from backend-builder
WORKDIR /app
COPY --from=backend-builder /app/backend/server ./backend/server

# Create a startup script that properly handles both processes
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'set -e' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Start backend in background' >> /app/start.sh && \
    echo 'cd /app && ./backend/server &' >> /app/start.sh && \
    echo 'BACKEND_PID=$!' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Start frontend in background' >> /app/start.sh && \
    echo 'cd /app/frontend && npm start &' >> /app/start.sh && \
    echo 'FRONTEND_PID=$!' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Function to handle shutdown' >> /app/start.sh && \
    echo 'cleanup() {' >> /app/start.sh && \
    echo '  echo "Shutting down services..."' >> /app/start.sh && \
    echo '  kill $BACKEND_PID 2>/dev/null || true' >> /app/start.sh && \
    echo '  kill $FRONTEND_PID 2>/dev/null || true' >> /app/start.sh && \
    echo '  wait' >> /app/start.sh && \
    echo '  exit 0' >> /app/start.sh && \
    echo '}' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Trap signals' >> /app/start.sh && \
    echo 'trap cleanup SIGTERM SIGINT' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Wait for processes' >> /app/start.sh && \
    echo 'wait' >> /app/start.sh && \
    chmod +x /app/start.sh

# Expose ports
EXPOSE 3000 8080

# Set environment variables
ENV NODE_ENV=production
ENV PORT=8080
ENV NEXT_PUBLIC_API_URL=http://localhost:8080

# Run startup script
CMD ["/app/start.sh"]
