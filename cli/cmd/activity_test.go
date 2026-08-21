package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendActivityRecordsFailureCause(t *testing.T) {
	c3Dir := t.TempDir()
	AppendActivity(c3Dir, "check", nil, false, false, "error: seal drift on c3-101\nhint: run c3x repair")

	entries, _ := readNewActivity(filepath.Join(c3Dir, activityFileName), 0)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Error, "seal drift") {
		t.Errorf("a failed row must carry its cause, got %q", entries[0].Error)
	}
}

func TestAppendActivityTruncatesLongCause(t *testing.T) {
	c3Dir := t.TempDir()
	AppendActivity(c3Dir, "check", nil, false, false, strings.Repeat("x", activityErrorMaxRunes*3))

	entries, _ := readNewActivity(filepath.Join(c3Dir, activityFileName), 0)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if len(entries[0].Error) > activityErrorMaxRunes {
		t.Errorf("an unbounded cause rotates the trail away: len = %d, cap = %d", len(entries[0].Error), activityErrorMaxRunes)
	}
}

func TestAppendActivityOmitsCauseOnSuccess(t *testing.T) {
	c3Dir := t.TempDir()
	AppendActivity(c3Dir, "list", nil, false, true, "")

	data, err := os.ReadFile(filepath.Join(c3Dir, activityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "error") {
		t.Errorf("a successful row must stay byte-identical to the pre-change format:\n%s", data)
	}
}
