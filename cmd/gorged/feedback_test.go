package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// postFeedback drives the handler the way the browser does: a multipart form.
// screenshot is attached only when non-nil, mirroring the client's optional
// attachment.
func postFeedback(t *testing.T, fs *feedbackStore, text, url string, screenshot []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if text != "" {
		if err := mw.WriteField("text", text); err != nil {
			t.Fatal(err)
		}
	}
	if url != "" {
		if err := mw.WriteField("url", url); err != nil {
			t.Fatal(err)
		}
	}
	if screenshot != nil {
		// The declared filename is deliberately hostile: the handler must
		// never let a submitted name reach the filesystem.
		w, err := mw.CreateFormFile("screenshot", "../../../etc/passwd")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(screenshot); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	fs.submit(rec, req)
	return rec
}

// storedReports returns every report directory the store has written.
func storedReports(t *testing.T, fs *feedbackStore) []string {
	t.Helper()
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(fs.dir, e.Name()))
		}
	}
	return out
}

// readReport decodes the report.json a submission wrote.
func readReport(t *testing.T, dir string) report {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func newTestStore(t *testing.T) *feedbackStore {
	t.Helper()
	fs, err := newFeedbackStore(filepath.Join(t.TempDir(), "feedback"))
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

// A text-only report is the ordinary case and must be stored whole, with the
// page it came from, because the URL is the part a later retelling loses.
func TestFeedbackStoresATextReport(t *testing.T) {
	fs := newTestStore(t)
	rec := postFeedback(t, fs, "Gisa made no zombies", "http://localhost:8080/t/t1?seat=0", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	dirs := storedReports(t, fs)
	if len(dirs) != 1 {
		t.Fatalf("stored %d reports, want 1", len(dirs))
	}
	got := readReport(t, dirs[0])
	if got.Text != "Gisa made no zombies" {
		t.Errorf("text = %q", got.Text)
	}
	if got.URL != "http://localhost:8080/t/t1?seat=0" {
		t.Errorf("url = %q", got.URL)
	}
	if got.Received == "" {
		t.Error("received timestamp is empty")
	}
	if got.Screenshot != "" {
		t.Errorf("screenshot = %q, want empty for a text-only report", got.Screenshot)
	}
}

// A report with no words is not a report: refusing it keeps the directory from
// filling with empty submissions from a mis-click.
func TestFeedbackRefusesAnEmptyReport(t *testing.T) {
	fs := newTestStore(t)
	if rec := postFeedback(t, fs, "   ", "", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if dirs := storedReports(t, fs); len(dirs) != 0 {
		t.Fatalf("stored %d reports, want 0", len(dirs))
	}
}

// A real PNG is stored under a name the SERVER chose. The submitted filename
// was "../../../etc/passwd"; nothing resembling it may reach the filesystem.
func TestFeedbackStoresAScreenshotUnderItsOwnName(t *testing.T) {
	fs := newTestStore(t)
	png := append(append([]byte{}, pngMagic...), []byte("IHDR and the rest")...)
	rec := postFeedback(t, fs, "wheel needs two clicks", "", png)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	dir := storedReports(t, fs)[0]
	if got := readReport(t, dir); got.Screenshot != "screenshot.png" {
		t.Fatalf("screenshot = %q, want %q", got.Screenshot, "screenshot.png")
	}
	stored, err := os.ReadFile(filepath.Join(dir, "screenshot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, png) {
		t.Error("stored screenshot bytes differ from what was submitted")
	}
	// The traversal the filename attempted must not exist anywhere.
	if _, err := os.Stat(filepath.Join(fs.dir, "..", "..", "..", "etc", "passwd")); err == nil {
		t.Fatal("a submitted filename escaped the feedback directory")
	}
}

// An attachment that is not a PNG is refused -- but the words survive. Losing
// a whole bug report because its screenshot was the wrong format would throw
// away the only part that cannot be recaptured later.
func TestFeedbackRejectsANonPNGButKeepsTheText(t *testing.T) {
	fs := newTestStore(t)
	rec := postFeedback(t, fs, "bolt hit my land", "", []byte("GIF89a not a png at all"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	dir := storedReports(t, fs)[0]
	got := readReport(t, dir)
	if got.Text != "bolt hit my land" {
		t.Errorf("text = %q, want the report to survive a refused attachment", got.Text)
	}
	if !strings.HasPrefix(got.Screenshot, "rejected:") {
		t.Errorf("screenshot = %q, want a recorded rejection", got.Screenshot)
	}
	if _, err := os.Stat(filepath.Join(dir, "screenshot.png")); err == nil {
		t.Error("a non-PNG attachment was written to disk")
	}
}

// Two reports submitted in the same second must not collide: the id carries
// random bytes precisely so a busy moment cannot overwrite a report.
func TestFeedbackIDsDoNotCollide(t *testing.T) {
	fs := newTestStore(t)
	for range 5 {
		if rec := postFeedback(t, fs, "same second", "", nil); rec.Code != http.StatusCreated {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if dirs := storedReports(t, fs); len(dirs) != 5 {
		t.Fatalf("stored %d reports, want 5 distinct directories", len(dirs))
	}
}
