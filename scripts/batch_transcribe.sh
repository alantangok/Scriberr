#!/bin/bash
# Batch Transcribe Audio Files with OpenAI GPT-4o
# Processes all files in a directory and merges SRT outputs
#
# Usage:
#   export OPENAI_API_KEY=your-key-here
#   ./batch_transcribe.sh /path/to/audio/directory --language yue --diarize

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT_DIR="$1"
shift  # Remove first arg, pass rest to Python script

if [ -z "$INPUT_DIR" ]; then
    echo "Usage: $0 <input_directory> [--language CODE] [--diarize]"
    echo ""
    echo "Options:"
    echo "  --language CODE  Language code (e.g., 'yue' for Cantonese)"
    echo "  --diarize        Enable speaker diarization"
    echo ""
    echo "Example:"
    echo "  export OPENAI_API_KEY=sk-..."
    echo "  $0 '/path/to/audio_split' --language yue --diarize"
    exit 1
fi

if [ -z "$OPENAI_API_KEY" ]; then
    echo "Error: OPENAI_API_KEY environment variable not set"
    exit 1
fi

if [ ! -d "$INPUT_DIR" ]; then
    echo "Error: Directory not found: $INPUT_DIR"
    exit 1
fi

# Check for Python script
PYTHON_SCRIPT="$SCRIPT_DIR/transcribe_openai.py"
if [ ! -f "$PYTHON_SCRIPT" ]; then
    echo "Error: transcribe_openai.py not found in $SCRIPT_DIR"
    exit 1
fi

# Find audio files
AUDIO_FILES=($(find "$INPUT_DIR" -maxdepth 1 -type f \( -name "*.mp3" -o -name "*.wav" -o -name "*.m4a" -o -name "*.flac" \) | sort))

if [ ${#AUDIO_FILES[@]} -eq 0 ]; then
    echo "No audio files found in $INPUT_DIR"
    exit 1
fi

echo "=========================================="
echo "Batch Transcription with OpenAI GPT-4o"
echo "=========================================="
echo "Input directory: $INPUT_DIR"
echo "Files to process: ${#AUDIO_FILES[@]}"
echo "Additional options: $@"
echo ""

# Process each file
COUNTER=0
TOTAL=${#AUDIO_FILES[@]}
ALL_SRT_FILES=()

for AUDIO_FILE in "${AUDIO_FILES[@]}"; do
    COUNTER=$((COUNTER + 1))
    BASENAME=$(basename "$AUDIO_FILE")

    echo ""
    echo "[$COUNTER/$TOTAL] Processing: $BASENAME"
    echo "------------------------------------------"

    # Transcribe
    python3 "$PYTHON_SCRIPT" "$AUDIO_FILE" --json "$@"

    # Track SRT file
    SRT_FILE="${AUDIO_FILE%.*}.srt"
    if [ -f "$SRT_FILE" ]; then
        ALL_SRT_FILES+=("$SRT_FILE")
    fi

    echo ""
done

# Merge SRT files if multiple
if [ ${#ALL_SRT_FILES[@]} -gt 1 ]; then
    echo ""
    echo "=========================================="
    echo "Merging ${#ALL_SRT_FILES[@]} SRT files..."
    echo "=========================================="

    MERGED_SRT="$INPUT_DIR/merged_transcript.srt"
    COUNTER=0
    TIME_OFFSET=0
    SEGMENT_NUM=1

    > "$MERGED_SRT"  # Clear/create file

    for SRT_FILE in "${ALL_SRT_FILES[@]}"; do
        echo "Adding: $(basename "$SRT_FILE")"

        # Get duration from corresponding JSON
        JSON_FILE="${SRT_FILE%.*}.json"
        if [ -f "$JSON_FILE" ]; then
            DURATION=$(python3 -c "import json; print(json.load(open('$JSON_FILE'))['duration'])" 2>/dev/null || echo "0")
        else
            DURATION=0
        fi

        # Add comment for segment
        echo "" >> "$MERGED_SRT"
        echo "# === Part $((COUNTER + 1)): $(basename "$SRT_FILE") ===" >> "$MERGED_SRT"
        echo "" >> "$MERGED_SRT"

        # Process SRT file (adjust timestamps would require more complex parsing)
        # For now, just concatenate with segment markers
        while IFS= read -r line || [ -n "$line" ]; do
            if [[ $line =~ ^[0-9]+$ ]]; then
                echo "$SEGMENT_NUM" >> "$MERGED_SRT"
                SEGMENT_NUM=$((SEGMENT_NUM + 1))
            else
                echo "$line" >> "$MERGED_SRT"
            fi
        done < "$SRT_FILE"

        COUNTER=$((COUNTER + 1))
        TIME_OFFSET=$(echo "$TIME_OFFSET + $DURATION" | bc 2>/dev/null || echo "$TIME_OFFSET")
    done

    echo ""
    echo "Merged SRT saved to: $MERGED_SRT"
fi

echo ""
echo "=========================================="
echo "Batch transcription complete!"
echo "=========================================="
echo "Processed: $TOTAL files"
echo "Output directory: $INPUT_DIR"

# List output files
echo ""
echo "Generated files:"
ls -la "$INPUT_DIR"/*.srt "$INPUT_DIR"/*.json 2>/dev/null || echo "No output files found"
