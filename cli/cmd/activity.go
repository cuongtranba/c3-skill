package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// activityEntry is one CLI action, appended to .c3/activity.jsonl after every
// command. The live explorer server tails this file: it is the single source
// of truth for "something happened" — mutating entries trigger a payload
// rebuild, every entry becomes an on-screen action event.
type activityEntry struct {
	Ts       string   `json:"ts"`
	Cmd      string   `json:"cmd"`
	Args     []string `json:"args,omitempty"`
	Mutating bool     `json:"mutating"`
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
}

const (
	activityFileName = "activity.jsonl"
	activityMaxBytes = 1 << 20 // 1 MB
	// The trail is trimmed by line count, not bytes, so an unbounded multi-line
	// error:/hint: cause would rotate recent history away.
	activityErrorMaxRunes = 400
	activityKeepLines     = 500
)

// AppendActivity records a CLI action and, on failure, its cause — the trail
// `c3x report fault` reads to ground a bug report in what actually happened.
// Best-effort: it never fails the command and writes nothing when the .c3
// directory does not exist.
func AppendActivity(c3Dir, command string, args []string, mutating, success bool, cause string) {
	if c3Dir == "" {
		return
	}
	if info, err := os.Stat(c3Dir); err != nil || !info.IsDir() {
		return
	}
	path := filepath.Join(c3Dir, activityFileName)
	truncateActivityIfLarge(path)

	entry := activityEntry{
		Ts:       time.Now().UTC().Format(time.RFC3339),
		Cmd:      command,
		Args:     args,
		Mutating: mutating,
		Success:  success,
	}
	if !success {
		entry.Error, _ = truncateRunes(cause, activityErrorMaxRunes)
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func truncateActivityIfLarge(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= activityMaxBytes {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) > activityKeepLines {
		lines = lines[len(lines)-activityKeepLines:]
	}
	kept := append(bytes.Join(lines, []byte("\n")), '\n')
	_ = os.WriteFile(path, kept, 0o644)
}

// readNewActivity returns entries appended past offset and the new offset.
// A shrunken file (truncation) resets the offset to zero first.
func readNewActivity(path string, offset int64) ([]activityEntry, int64) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, offset
	}
	if info.Size() < offset {
		offset = 0
	}
	if info.Size() == offset {
		return nil, offset
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()
	buf := make([]byte, info.Size()-offset)
	n, err := f.ReadAt(buf, offset)
	if err != nil && n == 0 {
		return nil, offset
	}
	buf = buf[:n]

	// Only consume complete lines; a partially-written trailing line waits
	// for the next poll.
	last := bytes.LastIndexByte(buf, '\n')
	if last < 0 {
		return nil, offset
	}
	consumed := buf[:last+1]
	var entries []activityEntry
	for _, line := range bytes.Split(bytes.TrimRight(consumed, "\n"), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e activityEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, offset + int64(len(consumed))
}
