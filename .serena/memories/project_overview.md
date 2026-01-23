# Scriberr Project Overview

## Purpose
Scriberr is an open-source, self-hosted audio transcription application designed for privacy and performance. It transcribes audio and video files locally without sending data to third-party cloud providers.

## Key Features
- **Local Transcription**: Uses ML models (NVIDIA Parakeet, Canary, Whisper) for offline transcription
- **Speaker Diarization**: Automatic speaker detection and labeling
- **LLM Integration**: Chat with transcripts via Ollama or OpenAI-compatible APIs
- **Folder Watcher**: Auto-processes new files for automation workflows
- **PWA Support**: Native app experience on desktop and mobile
- **Audio Recorder**: Built-in recording with note-taking features

## Tech Stack

### Backend (Go 1.24+)
- **Framework**: Gin (HTTP router)
- **Database**: SQLite (via glebarez/sqlite)
- **ORM**: GORM
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **CLI**: Cobra
- **Config**: Viper
- **Testing**: testify, gotestsum

### Frontend (React 19 + TypeScript)
- **Build Tool**: Vite 7
- **UI Library**: Radix UI components
- **Styling**: Tailwind CSS 4
- **State**: Zustand
- **Data Fetching**: TanStack Query
- **Audio**: wavesurfer.js
- **Markdown**: react-markdown with rehype plugins

### Infrastructure
- **Hot Reload (Go)**: Air
- **Linting**: golangci-lint (Go), ESLint (TypeScript)
- **Formatting**: Prettier
- **Git Hooks**: Lefthook
- **API Docs**: Swaggo/swag (OpenAPI)
- **Docker**: Multi-stage builds with CUDA support

## Repository Structure
```
scriberr/
├── cmd/
│   ├── server/main.go      # Main server entrypoint
│   └── scriberr-cli/        # CLI tool
├── internal/
│   ├── api/                 # HTTP handlers
│   ├── auth/                # Authentication
│   ├── config/              # Configuration
│   ├── database/            # Database setup
│   ├── llm/                 # LLM integrations
│   ├── models/              # Data models
│   ├── processing/          # Audio processing
│   ├── queue/               # Job queue
│   ├── repository/          # Data access layer
│   ├── service/             # Business logic
│   ├── transcription/       # Transcription adapters & pipeline
│   └── web/                 # Embedded frontend (dist/)
├── web/
│   ├── frontend/            # React application
│   └── project-site/        # Documentation website
├── tests/                   # Integration tests
├── data/                    # Runtime data (db, uploads, transcripts)
├── api-docs/                # Generated OpenAPI specs
└── PRPs/                    # Architecture decision records
```

## Key Architectural Patterns
- **Adapter Pattern**: Transcription services use adapters (OpenAI, WhisperX, etc.)
- **Pipeline Pattern**: Audio processing through splitter → transcription → merge
- **Repository Pattern**: Data access abstraction
- **Service Layer**: Business logic separation from handlers
- **Embedded Frontend**: React build is embedded in Go binary for single-binary distribution
