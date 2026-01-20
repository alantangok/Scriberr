#!/bin/bash
# Audio Splitter for OpenAI GPT-4o Transcription (25-minute limit)
# Usage: ./split_audio.sh <input_file> [segment_minutes]

set -e

INPUT_FILE="$1"
SEGMENT_MINUTES="${2:-24}"  # Default 24 minutes (safe margin for 25-min limit)
SEGMENT_SECONDS=$((SEGMENT_MINUTES * 60))

if [ -z "$INPUT_FILE" ]; then
    echo "Usage: $0 <input_audio_file> [segment_minutes]"
    echo "Example: $0 /path/to/audio.mp3 24"
    exit 1
fi

if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File not found: $INPUT_FILE"
    exit 1
fi

# Get file info
BASENAME=$(basename "$INPUT_FILE")
EXTENSION="${BASENAME##*.}"
FILENAME="${BASENAME%.*}"
DIR=$(dirname "$INPUT_FILE")
OUTPUT_DIR="${DIR}/${FILENAME}_split"

# Get duration
DURATION=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$INPUT_FILE" 2>/dev/null)
DURATION_INT=${DURATION%.*}
DURATION_MINS=$((DURATION_INT / 60))

echo "----------------------------------------"
echo "Audio Splitter for OpenAI GPT-4o"
echo "----------------------------------------"
echo "Input: $INPUT_FILE"
echo "Duration: ${DURATION_MINS} minutes"
echo "Segment size: ${SEGMENT_MINUTES} minutes"
echo "Output dir: $OUTPUT_DIR"
echo "----------------------------------------"

# Calculate number of segments
NUM_SEGMENTS=$(( (DURATION_INT + SEGMENT_SECONDS - 1) / SEGMENT_SECONDS ))
echo "Will create $NUM_SEGMENTS segments"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Split the file
echo "Splitting audio..."
ffmpeg -i "$INPUT_FILE" \
    -f segment \
    -segment_time $SEGMENT_SECONDS \
    -c copy \
    -reset_timestamps 1 \
    "${OUTPUT_DIR}/${FILENAME}_part%03d.${EXTENSION}" \
    -y 2>/dev/null

echo ""
echo "Done! Created files:"
ls -lh "$OUTPUT_DIR"
echo ""
echo "Total segments: $(ls -1 "$OUTPUT_DIR" | wc -l | tr -d ' ')"
echo "Output directory: $OUTPUT_DIR"
