package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/gorged"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// startGorged serves this binary's extensions over a fresh listener, persisting
// under dir, and returns the base URL once it answers. Cleanup stops the server.
func startGorged(t *testing.T, dir string, extra ...string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"-cards", testutil.CorpusDirOrSkip(t), "-decks", filepath.Join("..", "..", "testkit", "testutil", "decks"),
		"-tables", "1", "-seats", "2", "-pace", "0s", "-dir", dir, "-prewarm=false", "-feedback", ""}, extra...)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = gorged.Serve(ctx, args, ln, extensions())
		close(done)
	}()
	t.Cleanup(func() {
		stop()
		<-done
		if serveErr != nil && serveErr != context.Canceled {
			t.Errorf("serve: %v", serveErr)
		}
	})
	base := "http://" + ln.Addr().String()
	client := http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case <-done:
			t.Fatalf("server exited before it answered: %v", serveErr)
		default:
		}
		resp, err := client.Get(base + "/api/tables")
		if err == nil {
			resp.Body.Close()
			return base
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func capabilities(t *testing.T, base string) protocol.Capabilities {
	t.Helper()
	resp, err := http.Get(base + "/api/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/capabilities = %d, want 200", resp.StatusCode)
	}
	var caps protocol.Capabilities
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		t.Fatal(err)
	}
	return caps
}

// TestPublicBinaryServesDraftEvents: this binary links the draft-event plugin,
// so -events serves /api/events and says so. Without the flag the surface and
// the persistence directory are exactly a plain server's. It fails if the plugin
// is unlinked: -events would be refused at startup.
func TestPublicBinaryServesDraftEvents(t *testing.T) {
	t.Run("on", func(t *testing.T) {
		base := startGorged(t, t.TempDir(), "-events", "-humans", "1")
		if got := getStatus(t, base+"/api/events"); got != http.StatusOK {
			t.Errorf("/api/events = %d, want 200 with -events", got)
		}
		if !capabilities(t, base).Events {
			t.Error("capabilities say events:false with -events")
		}
	})
	t.Run("off", func(t *testing.T) {
		dir := t.TempDir()
		base := startGorged(t, dir)
		if got := getStatus(t, base+"/api/events"); got != http.StatusNotFound {
			t.Errorf("/api/events = %d, want 404 without -events", got)
		}
		if capabilities(t, base).Events {
			t.Error("capabilities say events:true without -events")
		}
		if _, err := os.Stat(filepath.Join(dir, "events")); !os.IsNotExist(err) {
			t.Errorf("<dir>/events exists without -events (stat err %v)", err)
		}
	})
}
