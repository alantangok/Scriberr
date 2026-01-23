# Code Style and Conventions

## Go Code Style

### General
- **Go version**: 1.24+
- **Linter**: golangci-lint with strict configuration
- **Module name**: `scriberr`

### Enabled Linters (.golangci.yml)
- errcheck, gosimple, govet, ineffassign, staticcheck, unused
- typecheck, goconst, gocyclo (complexity ≤ 25), revive

### Naming Conventions
- Use standard Go naming (camelCase for private, PascalCase for exported)
- Follow `var-naming`, `receiver-naming`, `error-naming` rules
- Error variables should follow Go conventions (`ErrSomething`)

### Code Organization
- Handlers in `internal/api/`
- Business logic in `internal/service/`
- Data access in `internal/repository/`
- Models in `internal/models/`
- Tests in `tests/` and alongside source files (`*_test.go`)

### Error Handling
- Always check and handle errors
- Use `error-return`, `errorf` patterns
- Wrap errors with context when appropriate

### Cyclomatic Complexity
- Maximum: 25 (enforced by gocyclo)
- Keep functions focused and small

## TypeScript/React Code Style

### General
- **TypeScript version**: ~5.8
- **React version**: 19.x
- **Build tool**: Vite 7
- **Linter**: ESLint with typescript-eslint

### Prettier Configuration (.prettierrc)
```json
{
  "useTabs": true,
  "singleQuote": true,
  "trailingComma": "none",
  "printWidth": 100
}
```

### ESLint Rules
- Based on recommended configs for JS, TypeScript, and React
- `no-console`: warn (allows warn, error, info)
- react-hooks recommended rules enabled
- react-refresh for HMR optimization

### Component Patterns
- Use functional components with hooks
- Radix UI for accessible primitives
- Tailwind CSS for styling (with tailwind-merge for class composition)
- Zustand for global state management
- TanStack Query for server state

### File Organization
```
web/frontend/src/
├── components/     # Reusable UI components
├── pages/          # Route-level components
├── hooks/          # Custom React hooks
├── lib/            # Utility functions
├── stores/         # Zustand stores
└── api/            # API client functions
```

## Git Conventions

### Pre-commit Hooks (Lefthook)
Runs in parallel:
1. `golangci-lint run ./...` for Go files
2. `npx eslint {staged_files}` for JS/TS
3. `npm run type-check` for TypeScript

### Commit Messages
Follow conventional commits:
- `feat:` new features
- `fix:` bug fixes
- `refactor:` code refactoring
- `style:` formatting changes
- `docs:` documentation
- `chore:` maintenance tasks
- `test:` adding/updating tests

### Branch Strategy
- Main branch: `main`
- Feature branches: `feature/description`
- Fix branches: `fix/description`

## API Documentation

### Swagger Annotations
Use swaggo annotations in handlers:
```go
// @Summary Get recording
// @Description Get a recording by ID
// @Tags recordings
// @Accept json
// @Produce json
// @Param id path int true "Recording ID"
// @Success 200 {object} models.Recording
// @Router /recordings/{id} [get]
```

Generate with: `make docs`

## Testing Conventions

### Go Tests
- Use testify for assertions
- Test files: `*_test.go`
- Integration tests in `tests/` directory
- Use `gotestsum` for formatted output

### Test Structure
```go
func TestFeatureName(t *testing.T) {
    // Arrange
    
    // Act
    
    // Assert
}
```

### Frontend Tests
- Component tests with React Testing Library (when configured)
- E2E tests with Playwright (optional)
