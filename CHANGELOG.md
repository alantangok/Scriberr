# Changelog

All notable changes to this project will be documented in this file.

## [0.3.0] - 20260123

### Added
- Automatic audio splitting for large files (>25MB or >25min) with 2-minute chunks
- HF_TOKEN environment variable fallback for diarization
- VAD segmentation thresholds for Pyannote diarization
- Auto-scroll and active segment highlighting in Timeline View
- Language support expansion to 58 languages for Whisper and OpenAI models
- Voxtral-mini transcription support with buffered transcription for long audio
- `make dev` command to replace dev.sh script

### Enhanced
- EmberPlayer memoization to prevent re-renders during playback
- Timeline View enabled for segment-level timestamps (OpenAI diarize)

### Changed
- Split merger logic into separate file for clean code compliance
- Transcription temp and output directories now configurable

### Fixed
- Handle gpt-4o-transcribe-diarize response format
- Listen button in selection menu now works in Timeline View
- Speaker rename now updates in real-time without page reload
- Keep header controls visible during auto-scroll
- API docs generation streamlined to sync both locations
- Auto device detection in Voxtral
- Use Literata font for all transcripts
- Disable timeline view for transcripts without word-level timestamps
- Voxtral model selection and dependencies
- Voxtral token limits based on 32k context window

### Refactored
- Merger logic separated into dedicated file

### Infrastructure
- Removed binary files from tracking
- Removed scripts/.venv from tracking and updated gitignore
- Updated PyAnnote test to reflect optional HF token

### Commit Range
From edab6cd to a340bfe (30 commits)

### Pending Changes
- None (working tree clean)
