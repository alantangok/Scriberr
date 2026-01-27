# Deployment Notes

## Frontend Embedding

The Go server embeds frontend static files from `internal/web/dist/` using `go:embed`.

### Critical Steps for Frontend Deployment:

1. **Build frontend**: `cd web/frontend && npm run build`
2. **Copy to embed location**: `cp -r web/frontend/dist/* internal/web/dist/`
3. **Build Go binary**: `GOOS=linux GOARCH=amd64 go build -o scriberr-linux ./cmd/server/main.go`
4. **Deploy**: Use `./deploy.sh --full`

### Automated Deployment:

The `deploy.sh` script handles full deployment but **does NOT** automatically copy frontend to `internal/web/dist`. 

You must run step 2 manually before deploying if frontend changed.

### Last Deployment:
- Date: 2026-01-28
- Feature: Click-to-seek on transcript text
- Frontend Bundle: `index-CA_b7XJr.js`
