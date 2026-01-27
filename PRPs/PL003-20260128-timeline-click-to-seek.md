# 🎯 Timeline Click-to-Seek Implementation Plan

## Overview
**Problem**: In expanded/timeline mode, clicking a segment requires holding Cmd/Ctrl — unintuitive for a timeline view where segments are natural seek targets.
**Solution**: Make the timestamp/speaker label area of each timeline segment a direct click-to-seek button (no modifier key needed), while preserving text selection on the segment text itself.

## Architecture Decision

**Approach**: Add a click handler on the timestamp/speaker column (`div.w-24`) that calls `onSeek(segment.start)` directly — no modifier key required. The text area keeps its existing Cmd/Ctrl click behavior for word-level seek.

This is the simplest approach because:
- The timestamp column is already `select-none` — no text selection conflict
- Each segment already has `segment.start` time available
- `onSeek` callback is already wired through the component tree
- No new components, hooks, or state needed

## Files to Change

1. `web/frontend/src/components/transcript/TranscriptView.tsx` (modify)
   - In `renderExpandedView()`, add `onClick={() => onSeek(segment.start)}` to the timestamp/speaker `div` (line ~312)
   - Add `cursor-pointer` class to that div
   - Add hover visual feedback (e.g., slight color change)

## Implementation Tasks

- [x] UI – Add onClick handler to timestamp/speaker div in expanded view segments
- [x] UI – Add cursor-pointer and hover styling to timestamp column
- [ ] UI – Optional: add a small play icon on hover for discoverability

## Success Criteria
- Click timestamp/speaker label in timeline view → audio seeks to segment start time
- Text selection in segment text body still works normally
- Cmd/Ctrl + click on text still does word-level seek
- No regressions on mobile (timestamp click works on mobile too)
