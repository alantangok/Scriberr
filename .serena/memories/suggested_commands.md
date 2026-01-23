# Suggested Commands

## Development

### Start Development Environment
```bash
# Full stack development (Go backend with Air + React frontend with Vite)
make dev

# Backend only (with live reload)
SKIP_NVIDIA_MODELS=true air

# Backend only (standard run)
SKIP_NVIDIA_MODELS=true go run cmd/server/main.go

# Frontend only
cd web/frontend && npm run dev
```

**Note**: Always use `SKIP_NVIDIA_MODELS=true` on machines without NVIDIA GPU to avoid large model downloads.

### Build
```bash
# Full production build (frontend embedded in Go binary)
make build

# Build CLI binaries for all platforms
make build-cli

# Build for Linux (production deployment)
GOOS=linux GOARCH=amd64 go build -o scriberr-linux ./cmd/server/main.go
```

## Testing

### Run Tests
```bash
# Run all tests with gotestsum
make test

# Run tests in watch mode
make test-watch

# Run specific test
go test -v ./tests/... -run TestName

# Run tests for a package
go test -v ./internal/transcription/...
```

## Linting & Formatting

### Go
```bash
# Lint Go code
golangci-lint run ./...

# Format Go code
gofmt -w .
```

### Frontend
```bash
# Lint TypeScript/React
cd web/frontend && npm run lint

# Type check
cd web/frontend && npm run type-check

# Format with Prettier
cd web/frontend && npx prettier --write .
```

## API Documentation

```bash
# Generate OpenAPI docs from Go annotations
make docs

# Clean generated docs
make docs-clean
```

## Documentation Website

```bash
# Start docs dev server
make website-dev

# Build docs for production
make website-build

# Preview built docs
make website-serve
```

## Git Hooks

Pre-commit hooks are configured via Lefthook and run automatically:
- `golangci-lint run ./...` for Go files
- `npx eslint` for JS/TS files
- `npm run type-check` for TypeScript

To manually install hooks:
```bash
lefthook install
```

## Docker

```bash
# Standard CPU deployment
docker compose up -d

# NVIDIA GPU deployment
docker compose -f docker-compose.cuda.yml up -d

# Build locally
docker compose -f docker-compose.build.yml up -d
```

## Production Deployment (likshing)

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o scriberr-linux ./cmd/server/main.go

# Copy binary
scp scriberr-linux likshing:/opt/scriberr/scriberr

# Copy frontend (if changed)
rsync -avz web/frontend/dist/ likshing:/opt/scriberr/web/frontend/dist/

# Restart service
ssh likshing "sudo systemctl restart scriberr"
```

## System Commands (macOS/Darwin)

```bash
# Git
git status
git log --oneline -10
git diff
git add -A && git commit -m "message"

# File operations
ls -la
find . -name "*.go" -type f
grep -r "pattern" --include="*.go"

# Process management
ps aux | grep scriberr
lsof -i :8080
```
