#!/bin/bash
# Auto Transcribe: Split + Transcribe + Merge SRT
# Automatically splits audio files > 24 minutes for OpenAI's 25-minute limit
#
# Usage:
#   export OPENAI_API_KEY=your-key-here
#   ./auto_transcribe.sh audio.mp3 --language yue --diarize

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT_FILE="$1"
shift || true  # Remove first arg, keep rest for Python script

SEGMENT_MINUTES=24  # Safe margin for 25-min limit
SEGMENT_SECONDS=$((SEGMENT_MINUTES * 60))

# Parse remaining args
PYTHON_ARGS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --segment-minutes)
            SEGMENT_MINUTES="$2"
            SEGMENT_SECONDS=$((SEGMENT_MINUTES * 60))
            shift 2
            ;;
        *)
            PYTHON_ARGS="$PYTHON_ARGS $1"
            shift
            ;;
    esac
done

# Validation
if [ -z "$INPUT_FILE" ]; then
    echo "Auto Transcribe: Split + Transcribe + Merge SRT"
    echo ""
    echo "Usage: $0 <audio_file> [options]"
    echo ""
    echo "Options:"
    echo "  --language CODE      Language code (e.g., 'yue' for Cantonese)"
    echo "  --diarize            Enable speaker diarization"
    echo "  --segment-minutes N  Segment length in minutes (default: 24)"
    echo ""
    echo "Example:"
    echo "  export OPENAI_API_KEY=sk-..."
    echo "  $0 '/path/to/long_meeting.mp3' --language yue --diarize"
    exit 1
fi

if [ -z "$OPENAI_API_KEY" ]; then
    echo "Error: OPENAI_API_KEY environment variable not set"
    echo "Run: export OPENAI_API_KEY=your-key-here"
    exit 1
fi

if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File not found: $INPUT_FILE"
    exit 1
fi

PYTHON_SCRIPT="$SCRIPT_DIR/transcribe_openai.py"
if [ ! -f "$PYTHON_SCRIPT" ]; then
    echo "Error: transcribe_openai.py not found"
    exit 1
fi

# Get file info
BASENAME=$(basename "$INPUT_FILE")
EXTENSION="${BASENAME##*.}"
FILENAME="${BASENAME%.*}"
DIR=$(dirname "$INPUT_FILE")
OUTPUT_DIR="${DIR}/${FILENAME}_transcription"

# Get duration
DURATION=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$INPUT_FILE" 2>/dev/null)
DURATION_INT=${DURATION%.*}
DURATION_MINS=$((DURATION_INT / 60))

echo "=========================================="
echo "Auto Transcribe"
echo "=========================================="
echo "Input: $INPUT_FILE"
echo "Duration: ${DURATION_MINS} minutes"
echo "Max segment: ${SEGMENT_MINUTES} minutes"
echo "Options: $PYTHON_ARGS"
echo "=========================================="
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Determine if splitting is needed
if [ $DURATION_INT -gt $SEGMENT_SECONDS ]; then
    NUM_SEGMENTS=$(( (DURATION_INT + SEGMENT_SECONDS - 1) / SEGMENT_SECONDS ))
    echo "Audio exceeds ${SEGMENT_MINUTES} minutes. Splitting into $NUM_SEGMENTS parts..."
    echo ""

    # Split audio
    SPLIT_DIR="$OUTPUT_DIR/split_audio"
    mkdir -p "$SPLIT_DIR"

    ffmpeg -i "$INPUT_FILE" \
        -f segment \
        -segment_time $SEGMENT_SECONDS \
        -c copy \
        -reset_timestamps 1 \
        "${SPLIT_DIR}/${FILENAME}_part%03d.${EXTENSION}" \
        -y 2>/dev/null

    echo "Split complete. Parts:"
    ls -la "$SPLIT_DIR"
    echo ""

    # Get sorted list of split files
    AUDIO_FILES=($(find "$SPLIT_DIR" -maxdepth 1 -type f -name "*.${EXTENSION}" | sort))
else
    echo "Audio is under ${SEGMENT_MINUTES} minutes. No splitting needed."
    AUDIO_FILES=("$INPUT_FILE")
fi

# Transcribe each part
COUNTER=0
TOTAL=${#AUDIO_FILES[@]}
ALL_TRANSCRIPTS=()
CUMULATIVE_OFFSET=0

echo ""
echo "=========================================="
echo "Transcribing ${TOTAL} file(s)..."
echo "=========================================="

for AUDIO_FILE in "${AUDIO_FILES[@]}"; do
    COUNTER=$((COUNTER + 1))
    PART_BASENAME=$(basename "$AUDIO_FILE")

    echo ""
    echo "[$COUNTER/$TOTAL] $PART_BASENAME"
    echo "------------------------------------------"

    # Get part duration for offset calculation
    PART_DURATION=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$AUDIO_FILE" 2>/dev/null)
    PART_DURATION_INT=${PART_DURATION%.*}

    # Output paths
    PART_NAME="${PART_BASENAME%.*}"
    PART_SRT="$OUTPUT_DIR/${PART_NAME}.srt"
    PART_JSON="$OUTPUT_DIR/${PART_NAME}.json"

    # Transcribe
    python3 "$PYTHON_SCRIPT" "$AUDIO_FILE" --output "$PART_SRT" --json $PYTHON_ARGS

    # Store for merging
    ALL_TRANSCRIPTS+=("$PART_SRT:$CUMULATIVE_OFFSET")

    # Update offset for next part
    CUMULATIVE_OFFSET=$((CUMULATIVE_OFFSET + PART_DURATION_INT))
done

# Merge SRT files with proper timestamp adjustment
if [ ${#ALL_TRANSCRIPTS[@]} -gt 1 ]; then
    echo ""
    echo "=========================================="
    echo "Merging transcripts with adjusted timestamps..."
    echo "=========================================="

    FINAL_SRT="$OUTPUT_DIR/${FILENAME}_full.srt"

    # Python script for proper SRT merging with timestamp adjustment
    python3 << 'MERGE_SCRIPT' "$FINAL_SRT" "${ALL_TRANSCRIPTS[@]}"
import sys
import re
from datetime import timedelta

def parse_srt_time(time_str):
    """Parse SRT timestamp to seconds"""
    match = re.match(r'(\d{2}):(\d{2}):(\d{2}),(\d{3})', time_str)
    if match:
        h, m, s, ms = map(int, match.groups())
        return h * 3600 + m * 60 + s + ms / 1000
    return 0

def format_srt_time(seconds):
    """Format seconds to SRT timestamp"""
    td = timedelta(seconds=seconds)
    hours, remainder = divmod(td.total_seconds(), 3600)
    minutes, secs = divmod(remainder, 60)
    ms = int((secs % 1) * 1000)
    return f"{int(hours):02d}:{int(minutes):02d}:{int(secs):02d},{ms:03d}"

def merge_srt_files(output_path, file_offset_pairs):
    """Merge multiple SRT files with timestamp offsets"""
    merged_entries = []
    global_counter = 1

    for pair in file_offset_pairs:
        srt_path, offset_str = pair.split(':')
        offset = int(offset_str)

        try:
            with open(srt_path, 'r', encoding='utf-8') as f:
                content = f.read()
        except FileNotFoundError:
            print(f"Warning: {srt_path} not found, skipping")
            continue

        # Parse SRT entries
        entries = re.split(r'\n\n+', content.strip())
        for entry in entries:
            lines = entry.strip().split('\n')
            if len(lines) >= 3:
                # Skip index line, parse timestamps
                time_match = re.match(r'(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})', lines[1])
                if time_match:
                    start = parse_srt_time(time_match.group(1)) + offset
                    end = parse_srt_time(time_match.group(2)) + offset
                    text = '\n'.join(lines[2:])

                    merged_entries.append({
                        'index': global_counter,
                        'start': format_srt_time(start),
                        'end': format_srt_time(end),
                        'text': text
                    })
                    global_counter += 1

    # Write merged SRT
    with open(output_path, 'w', encoding='utf-8') as f:
        for entry in merged_entries:
            f.write(f"{entry['index']}\n")
            f.write(f"{entry['start']} --> {entry['end']}\n")
            f.write(f"{entry['text']}\n\n")

    print(f"Merged {len(merged_entries)} segments into {output_path}")

if __name__ == '__main__':
    output_path = sys.argv[1]
    file_offset_pairs = sys.argv[2:]
    merge_srt_files(output_path, file_offset_pairs)
MERGE_SCRIPT

    echo ""
    echo "Final merged SRT: $FINAL_SRT"
else
    # Single file, just copy
    FINAL_SRT="$OUTPUT_DIR/${FILENAME}_full.srt"
    cp "${ALL_TRANSCRIPTS[0]%%:*}" "$FINAL_SRT"
fi

# Summary
echo ""
echo "=========================================="
echo "Transcription Complete!"
echo "=========================================="
echo "Input: $INPUT_FILE"
echo "Duration: ${DURATION_MINS} minutes"
echo "Parts transcribed: $TOTAL"
echo ""
echo "Output files:"
ls -la "$OUTPUT_DIR"/*.srt 2>/dev/null || echo "No SRT files"
echo ""
echo "Final SRT: $FINAL_SRT"

# Cleanup option
if [ -d "$OUTPUT_DIR/split_audio" ]; then
    echo ""
    echo "Note: Split audio files kept in: $OUTPUT_DIR/split_audio"
    echo "To delete: rm -rf '$OUTPUT_DIR/split_audio'"
fi
