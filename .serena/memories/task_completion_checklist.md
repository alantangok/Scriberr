# Task Completion Checklist

## Before Submitting Code

### 1. Linting (Required)
```bash
# Go linting
golangci-lint run ./...

# Frontend linting
cd web/frontend && npm run lint

# Frontend type checking
cd web/frontend && npm run type-check
```

### 2. Testing (Required for code changes)
```bash
# Run all tests
make test

# Or run specific tests
go test -v ./internal/...
go test -v ./tests/...
```

### 3. Build Verification (For significant changes)
```bash
# Full production build
make build

# Verify binary runs
SKIP_NVIDIA_MODELS=true ./scriberr
```

## Pre-commit Hooks (Automatic)

Lefthook automatically runs on commit:
- `golangci-lint run ./...` (Go)
- `npx eslint` (TypeScript/React)
- `npm run type-check` (TypeScript)

If hooks fail, fix issues and recommit.

## API Changes

When modifying API endpoints:
```bash
# Regenerate OpenAPI documentation
make docs
```

## Frontend Changes

When modifying frontend:
```bash
# Lint and type-check
cd web/frontend
npm run lint
npm run type-check

# Build to ensure no compilation errors
npm run build
```

## Database Changes

When modifying models:
- Ensure GORM migrations are handled
- Test with fresh database if needed
- Consider backwards compatibility

## Documentation

- Update CLAUDE.md if adding new commands or workflows
- Update README.md for user-facing changes
- Add PRPs for significant architectural decisions

## Deployment Checklist

### Local Testing
1. Start dev server: `make dev`
2. Test affected features
3. Run full test suite: `make test`

### Production (likshing)
1. Build Linux binary: `GOOS=linux GOARCH=amd64 go build -o scriberr-linux ./cmd/server/main.go`
2. If frontend changed: `cd web/frontend && npm run build`
3. Deploy:
   ```bash
   scp scriberr-linux likshing:/opt/scriberr/scriberr
   rsync -avz web/frontend/dist/ likshing:/opt/scriberr/web/frontend/dist/  # if needed
   ssh likshing "sudo systemctl restart scriberr"
   ```
4. Verify: Check https://scriberr.hachitg4ever.com
