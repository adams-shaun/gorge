package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// feedbackStore accepts a player's own bug report — the FEEDBACK affordance in
// the client — and writes it to a local directory the operator can read.
//
// It exists because the useful bugs in this engine are the ones somebody hits
// while actually playing (a card that makes zero tokens, a burn spell that can
// target a land), and those arrive today only as a message typed somewhere
// outside the system, if at all. A report captured at the table carries the
// one thing a later retelling loses: the URL, and therefore the table, the
// match and the seat it happened at.
//
// Nothing here trusts the client. The report is written under a directory this
// server chose, into a filename this server generated: the submitter names
// neither. A multipart field is read through a size cap rather than into
// memory unbounded, and an attached screenshot is accepted only if its own
// leading bytes are a PNG — a client-declared content type is not evidence.
type feedbackStore struct {
	dir string
}

// maxFeedbackBytes caps a whole submission, text and screenshot together. A
// full-screen PNG at a normal desktop resolution is comfortably under this;
// anything above it is not a bug report.
const maxFeedbackBytes = 12 << 20 // 12 MiB

// maxFeedbackTextBytes caps the prose half on its own, so a caller cannot
// spend the whole budget on a text field.
const maxFeedbackTextBytes = 16 << 10 // 16 KiB

// pngMagic is the 8-byte PNG signature (PNG spec 5.2). An attachment whose
// first bytes are not this is refused, whatever the part claims to be.
var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// newFeedbackStore prepares the directory reports are written to. Failing here
// is a startup error rather than a surprise at the first submission.
func newFeedbackStore(dir string) (*feedbackStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("feedback directory %s: %w", dir, err)
	}
	return &feedbackStore{dir: dir}, nil
}

// report is the JSON written beside any screenshot. Received is the server's
// own clock, not the client's: a timestamp a submitter controls is not a
// timestamp.
type report struct {
	Received   string `json:"received"`
	Text       string `json:"text"`
	URL        string `json:"url,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	Screenshot string `json:"screenshot,omitempty"`
}

// submit handles POST /api/feedback: a multipart form with a required `text`
// part, an optional `url` part naming the page the reporter was on, and an
// optional `screenshot` part.
func (fs *feedbackStore) submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFeedbackBytes)
	if err := r.ParseMultipartForm(maxFeedbackBytes); err != nil {
		http.Error(w, "feedback: malformed or oversized submission", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "feedback: text is required", http.StatusBadRequest)
		return
	}
	if len(text) > maxFeedbackTextBytes {
		text = text[:maxFeedbackTextBytes]
	}

	id, err := feedbackID()
	if err != nil {
		http.Error(w, "feedback: could not allocate a report id", http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(fs.dir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "feedback: could not store the report", http.StatusInternalServerError)
		return
	}

	rep := report{
		Received:  time.Now().UTC().Format(time.RFC3339),
		Text:      text,
		URL:       strings.TrimSpace(r.FormValue("url")),
		UserAgent: r.UserAgent(),
	}

	// The screenshot is optional, and a bad one must not lose the words: a
	// report whose attachment is refused is still stored, with the reason
	// recorded rather than a 400 that throws the text away.
	if f, _, err := r.FormFile("screenshot"); err == nil {
		defer func() { _ = f.Close() }()
		switch name, err := fs.writePNG(dir, f); {
		case err != nil:
			rep.Screenshot = "rejected: " + err.Error()
		default:
			rep.Screenshot = name
		}
	}

	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		http.Error(w, "feedback: could not encode the report", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), append(body, '\n'), 0o644); err != nil {
		http.Error(w, "feedback: could not store the report", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(os.Stderr, "gorged: feedback %s stored in %s\n", id, dir)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// writePNG stores the attachment under a name this server chose, after
// checking the bytes really begin with the PNG signature. The submitted
// filename is never consulted, so no part of it can reach the filesystem.
func (fs *feedbackStore) writePNG(dir string, src io.Reader) (string, error) {
	head := make([]byte, len(pngMagic))
	if _, err := io.ReadFull(src, head); err != nil {
		return "", fmt.Errorf("not a PNG")
	}
	for i, b := range pngMagic {
		if head[i] != b {
			return "", fmt.Errorf("not a PNG")
		}
	}
	const name = "screenshot.png"
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("could not be stored")
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(head); err != nil {
		return "", fmt.Errorf("could not be stored")
	}
	if _, err := io.Copy(f, src); err != nil {
		return "", fmt.Errorf("could not be stored")
	}
	return name, nil
}

// feedbackID is a sortable, collision-resistant directory name: the UTC
// timestamp so a listing reads chronologically, plus random bytes so two
// reports in the same second cannot land in the same directory.
func feedbackID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:]), nil
}
