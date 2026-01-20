#!/usr/bin/env python3
"""
OpenAI GPT-4o Transcription Script with SRT Output
Supports speaker diarization and Cantonese

Usage:
    export OPENAI_API_KEY=your-key-here
    python transcribe_openai.py audio.mp3 --language yue --diarize

Requirements:
    pip install openai
"""

import argparse
import os
import sys
import json
from pathlib import Path
from datetime import timedelta

try:
    from openai import OpenAI
except ImportError:
    print("Error: openai package not installed. Run: pip install openai")
    sys.exit(1)


def format_timestamp(seconds: float) -> str:
    """Convert seconds to SRT timestamp format (HH:MM:SS,mmm)"""
    td = timedelta(seconds=seconds)
    hours, remainder = divmod(td.total_seconds(), 3600)
    minutes, seconds = divmod(remainder, 60)
    milliseconds = int((seconds % 1) * 1000)
    return f"{int(hours):02d}:{int(minutes):02d}:{int(seconds):02d},{milliseconds:03d}"


def transcribe_audio(
    file_path: str,
    language: str = None,
    diarize: bool = False,
    model: str = "gpt-4o-transcribe"
) -> dict:
    """Transcribe audio using OpenAI API"""

    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY environment variable not set")

    client = OpenAI(api_key=api_key)

    # Choose model based on diarization
    if diarize:
        model = "gpt-4o-transcribe-diarize"

    print(f"Transcribing: {file_path}")
    print(f"Model: {model}")
    print(f"Language: {language or 'auto-detect'}")
    print(f"Diarization: {diarize}")
    print("-" * 40)

    with open(file_path, "rb") as audio_file:
        params = {
            "model": model,
            "file": audio_file,
            "response_format": "verbose_json",
        }

        if language:
            params["language"] = language

        # Request word-level timestamps
        if not diarize:
            params["timestamp_granularities"] = ["word", "segment"]

        response = client.audio.transcriptions.create(**params)

    return response


def generate_srt(response, output_path: str, use_words: bool = False):
    """Generate SRT file from transcription response"""

    srt_lines = []
    counter = 1

    # Use segments (or words if requested)
    if hasattr(response, 'segments') and response.segments:
        for segment in response.segments:
            start = format_timestamp(segment.start)
            end = format_timestamp(segment.end)
            text = segment.text.strip()

            # Add speaker label if available (from diarization)
            if hasattr(segment, 'speaker') and segment.speaker:
                text = f"[{segment.speaker}] {text}"

            srt_lines.append(f"{counter}")
            srt_lines.append(f"{start} --> {end}")
            srt_lines.append(text)
            srt_lines.append("")
            counter += 1

    elif hasattr(response, 'words') and response.words and use_words:
        # Group words into chunks for SRT
        chunk_size = 10
        words = response.words
        for i in range(0, len(words), chunk_size):
            chunk = words[i:i + chunk_size]
            start = format_timestamp(chunk[0].start)
            end = format_timestamp(chunk[-1].end)
            text = " ".join(w.word for w in chunk)

            srt_lines.append(f"{counter}")
            srt_lines.append(f"{start} --> {end}")
            srt_lines.append(text)
            srt_lines.append("")
            counter += 1

    else:
        # Fallback: single segment with full text
        srt_lines.append("1")
        srt_lines.append(f"00:00:00,000 --> 00:99:59,999")
        srt_lines.append(response.text)
        srt_lines.append("")

    with open(output_path, "w", encoding="utf-8") as f:
        f.write("\n".join(srt_lines))

    print(f"SRT saved to: {output_path}")
    return counter - 1


def main():
    parser = argparse.ArgumentParser(
        description="Transcribe audio using OpenAI GPT-4o with SRT output"
    )
    parser.add_argument("audio_file", help="Path to audio file (mp3, wav, etc.)")
    parser.add_argument(
        "--language", "-l",
        help="Language code (e.g., 'yue' for Cantonese, 'en' for English)"
    )
    parser.add_argument(
        "--diarize", "-d",
        action="store_true",
        help="Enable speaker diarization"
    )
    parser.add_argument(
        "--output", "-o",
        help="Output file path (default: same as input with .srt extension)"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Also save raw JSON response"
    )
    parser.add_argument(
        "--model", "-m",
        default="gpt-4o-transcribe",
        help="Model to use (default: gpt-4o-transcribe)"
    )

    args = parser.parse_args()

    # Validate input file
    audio_path = Path(args.audio_file)
    if not audio_path.exists():
        print(f"Error: File not found: {audio_path}")
        sys.exit(1)

    # Set output path
    if args.output:
        output_path = Path(args.output)
    else:
        output_path = audio_path.with_suffix(".srt")

    try:
        # Transcribe
        response = transcribe_audio(
            str(audio_path),
            language=args.language,
            diarize=args.diarize,
            model=args.model
        )

        # Save JSON if requested
        if args.json:
            json_path = output_path.with_suffix(".json")
            with open(json_path, "w", encoding="utf-8") as f:
                # Convert response to dict
                json.dump(response.model_dump(), f, ensure_ascii=False, indent=2)
            print(f"JSON saved to: {json_path}")

        # Generate SRT
        segment_count = generate_srt(response, str(output_path))

        print("-" * 40)
        print(f"Transcription complete!")
        print(f"Language detected: {response.language}")
        print(f"Duration: {response.duration:.1f}s")
        print(f"Segments: {segment_count}")
        print(f"Full text preview: {response.text[:200]}...")

    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
