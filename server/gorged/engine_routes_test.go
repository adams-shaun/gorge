package gorged

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// routePattern finds a mux registration, with or without a method.
var routePattern = regexp.MustCompile(`Handle(?:Func)?\("(?:(GET|POST) )?(/[^"]*)"`)

// registeredRoutes is every pattern the sources register under dir, with prefix in
// front. It reads the sources so a route added later is driven without a list to
// remember.
func registeredRoutes(t *testing.T, dir, prefix string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range routePattern.FindAllStringSubmatch(string(raw), -1) {
			out = append(out, strings.TrimSpace(m[1]+" "+prefix+m[2]))
		}
	}
	return out
}

// routeRequest is one request a sweep sends for a pattern.
func routeRequest(ctx context.Context, base, pattern, table, token string) (*http.Request, error) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		method, path = http.MethodGet, pattern
	}
	path = strings.NewReplacer("{t}", table, "{k}", "1", "{id}", "1", "{key}", "x").Replace(path)
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path+"?seat=0&seq=1&sub=t/1/0", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// sweepRoutes sends every route one request. A panic in a handler reaches the client
// as a dropped connection, and in a goroutine it ends the test binary.
func sweepRoutes(t *testing.T, base, table, token string, routes []string) {
	t.Helper()
	for _, pattern := range routes {
		if pattern == "/" { // the web bundle, absent from a test server
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		req, err := routeRequest(ctx, base, pattern, table, token)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_, err = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= http.StatusInternalServerError {
				t.Errorf("%s: status %d", pattern, resp.StatusCode)
			}
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: %v", pattern, err)
		}
		cancel()
	}
}

// fileFeedback posts a report naming the table and returns report.json's snapshot
// status and the report's directory.
func fileFeedback(t *testing.T, base, dir, table string) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range map[string]string{"text": "a report", "table": table, "seat": "0"} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	resp, err := http.Post(base+"/api/feedback", w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("feedback: status %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	reportDir := filepath.Join(dir, created.ID)
	raw, err := os.ReadFile(filepath.Join(reportDir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rep report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	return rep.Snapshot, reportDir
}

// concede ends the game for seat 0, waiting for its priority.
func concede(t *testing.T, match, token string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, match+"/pending?seat=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Seq     uint64 `json:"seq"`
			Player  int    `json:"player"`
			Kind    string `json:"kind"`
			Min     int    `json:"min"`
			Options []struct {
				Index int    `json:"index"`
				Kind  string `json:"kind"`
			} `json:"options"`
		}
		err = json.NewDecoder(resp.Body).Decode(&d)
		resp.Body.Close()
		if err != nil || d.Kind == "" || d.Player != 0 {
			time.Sleep(time.Second / 100)
			continue
		}
		choices := []int{}
		for i := 0; i < d.Min; i++ {
			choices = append(choices, i)
		}
		done := false
		for _, o := range d.Options {
			if o.Kind == "concede" {
				choices, done = []int{o.Index}, true
			}
		}
		raw, _ := json.Marshal(map[string]any{"seq": d.Seq, "player": 0, "choices": choices})
		req, _ = http.NewRequest(http.MethodPost, match+"/intent?seat=0", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if done {
			return
		}
	}
	t.Fatal("seat 0 never got priority")
}

// TestEveryRouteAnswersOnADiracTable drives every route a server registers, on a live
// dirac table and on the same table's finished match.
func TestEveryRouteAnswersOnADiracTable(t *testing.T) {
	c := diracConfig()
	c.vsbot, c.botPolicy, c.manabrew = true, "bot", true
	c.feedback = t.TempDir()
	base, cancel, _ := startServe(t, c)
	defer cancel()
	status, body := postGame(t, base, map[string]string{"format": "constructed", "bot_policy": "bot"})
	if status != http.StatusOK {
		t.Fatalf("create: %d %s", status, body)
	}
	var g httpapi.CreateGameResponse
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	routes := registeredRoutes(t, "../host/httpapi", "")
	routes = append(routes, registeredRoutes(t, "../host/manabrewhttp", manabrewRoutePrefix)...)
	if len(routes) < 30 {
		t.Fatalf("found %d routes", len(routes))
	}

	sweepRoutes(t, base, g.Table, g.Token, routes)
	concede(t, fmt.Sprintf("%s/api/tables/%s/matches/1", base, g.Table), g.Token)
	sweepRoutes(t, base, g.Table, g.Token, routes)
	if snap, _ := fileFeedback(t, base, c.feedback, g.Table); !strings.HasPrefix(snap, "captured") {
		t.Fatalf("a finished game's report: %s", snap)
	}
}
