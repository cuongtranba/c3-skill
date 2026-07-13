package cmd

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendActivity_WritesAndTailReads(t *testing.T) {
	dir := t.TempDir()
	AppendActivity(dir, "read", []string{"c3-101"}, false, true)
	AppendActivity(dir, "add", []string{"component", "x"}, true, true)
	AppendActivity(dir, "set", []string{"adr-x", "status"}, true, false)

	path := filepath.Join(dir, activityFileName)
	entries, offset := readNewActivity(path, 0)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Cmd != "read" || entries[0].Mutating || !entries[0].Success {
		t.Errorf("first entry wrong: %+v", entries[0])
	}
	if entries[2].Cmd != "set" || !entries[2].Mutating || entries[2].Success {
		t.Errorf("third entry wrong: %+v", entries[2])
	}

	// Nothing new past the offset.
	more, _ := readNewActivity(path, offset)
	if len(more) != 0 {
		t.Fatalf("expected no new entries, got %d", len(more))
	}

	// A shrunken file (truncation) resets and re-reads.
	if err := os.WriteFile(path, []byte(`{"ts":"t","cmd":"init","mutating":true,"success":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := readNewActivity(path, offset)
	if len(after) != 1 || after[0].Cmd != "init" {
		t.Fatalf("truncation reset failed: %+v", after)
	}
}

func TestAppendActivity_MissingDirIsNoop(t *testing.T) {
	AppendActivity(filepath.Join(t.TempDir(), "nope"), "read", nil, false, true)
}

func TestReadNewActivity_PartialLineWaits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, activityFileName)
	if err := os.WriteFile(path, []byte(`{"ts":"t","cmd":"read","mutating":false,"success":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, offset := readNewActivity(path, 0)
	if len(entries) != 0 || offset != 0 {
		t.Fatalf("partial line must not be consumed: entries=%d offset=%d", len(entries), offset)
	}
}

func TestHandleEvents_SendsCurrentPayloadFirst(t *testing.T) {
	srv := &exploreServer{hub: newSSEHub(`{"project":"t"}`)}
	ts := httptest.NewServer(http.HandlerFunc(srv.handleEvents))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("wrong content type %q", ct)
	}

	reader := bufio.NewReader(resp.Body)
	line1, _ := reader.ReadString('\n')
	line2, _ := reader.ReadString('\n')
	if !strings.HasPrefix(line1, "event: payload") {
		t.Fatalf("first frame must be the current payload, got %q", line1)
	}
	if !strings.Contains(line2, `{"project":"t"}`) {
		t.Fatalf("payload data missing, got %q", line2)
	}

	// A broadcast reaches the connected client.
	done := make(chan string, 1)
	go func() {
		reader.ReadString('\n') // blank line after first frame
		l, _ := reader.ReadString('\n')
		d, _ := reader.ReadString('\n')
		done <- l + d
	}()
	time.Sleep(50 * time.Millisecond)
	srv.hub.broadcast("action", `{"cmd":"add"}`)
	select {
	case got := <-done:
		if !strings.Contains(got, "event: action") || !strings.Contains(got, `{"cmd":"add"}`) {
			t.Fatalf("broadcast frame wrong: %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for broadcast frame")
	}
}

func TestInjectExplorerPayload_LiveFlag(t *testing.T) {
	shell := "<script>/*__C3_DATA__*/ /*__C3_LIVE__*/</script>"
	p := validExplorePayload()

	static, err := injectExplorerPayload(shell, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(static, "window.C3_LIVE") {
		t.Error("static mode must not set window.C3_LIVE")
	}
	live, err := injectExplorerPayload(shell, p, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(live, "window.C3_LIVE = true;") {
		t.Error("live mode must set window.C3_LIVE")
	}
	if !strings.Contains(live, "window.C3_DATA = ") {
		t.Error("live mode must still inject the payload")
	}
}
