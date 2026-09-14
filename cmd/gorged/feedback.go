package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/state"
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
//
// When the report names a table this server serves — via its url's /t/<id>
// route, or explicit `table`/`seat` form fields — the store also captures a
// replayable snapshot of that table's current match beside the report
// (feedback.go's host half): match.json (the sidecar plus deck contents),
// log.json (the event log as of the submit) and view.json (the reporting
// seat's own redacted view). Snapshot failure never loses the report: the
// reason is recorded in report.json's snapshot field and the submission
// still returns 201, the same rule as a rejected screenshot.
type feedbackStore struct {
	dir string
	reg *host.Registry
}

// maxFeedbackBytes caps a whole submission, text and screenshot together. A
// full-screen PNG at a normal desktop resolution is comfortably under this;
// anything above it is not a bug report.
const maxFeedbackBytes = 12 << 20 // 12 MiB

// maxFeedbackTextBytes caps the prose half on its own, so a caller cannot
// spend the whole budget on a text field.
const maxFeedbackTextBytes = 16 << 10 // 16 KiB

// maxFeedbackSnapshotBytes caps the three snapshot files' combined size.
// The log dominates: measured over 189 completed four-seat Commander
// matches (the m38 repo decks, played out at full speed), the largest log
// marshalled to 565,993 bytes of JSON (typical 100-200 KB) and the largest
// four-seat constructed log to 451,493 — a real commander log is tens of
// KB to a fraction of a MB, because events marshal compactly (~15 bytes
// each). 64 MiB is over a hundred times the measured worst shape, so it
// leaves room for a far longer human game while still bounding what one
// submission can leave on disk. A snapshot over the cap is refused whole
// (a truncated log would not replay) and the reason recorded, never
// silently clipped.
const maxFeedbackSnapshotBytes = 64 << 20 // 64 MiB

// pngMagic is the 8-byte PNG signature (PNG spec 5.2). An attachment whose
// first bytes are not this is refused, whatever the part claims to be.
var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// newFeedbackStore prepares the directory reports are written to. reg is
// the registry the snapshot capture reads (SnapshotForFeedback); it may be
// nil in tests that exercise only the report half. Failing here
// is a startup error rather than a surprise at the first submission.
func newFeedbackStore(dir string, reg *host.Registry) (*feedbackStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("feedback directory %s: %w", dir, err)
	}
	return &feedbackStore{dir: dir, reg: reg}, nil
}

// report is the JSON written beside any screenshot. Received is the server's
// own clock, not the client's: a timestamp a submitter controls is not a
// timestamp. Snapshot says what the capture produced — "captured" with the
// files written, or "unavailable: <reason>" / "partial: <reason>" when some
// or all of it could not be. A report that names no table records that fact
// as unavailable rather than silently omitting the snapshot status.
type report struct {
	Received   string `json:"received"`
	Text       string `json:"text"`
	URL        string `json:"url,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	Screenshot string `json:"screenshot,omitempty"`
	Snapshot   string `json:"snapshot,omitempty"`
}

// submit handles POST /api/feedback: a multipart form with a required `text`
// part, an optional `url` part naming the page the reporter was on, an
// optional `screenshot` part, and — when the client knows them — optional
// `table` and `seat` parts naming the table the report is about (the url's
// own /t/<id> route and seat= query parameter are read when the fields are
// absent).
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

	// The snapshot is captured before report.json is written, so the report
	// can say what the capture produced. It is best-effort in both halves:
	// a panic inside the snapshot path is recovered into an unavailable
	// reason, and a report whose table is unknown is still stored whole —
	// the words are the part that cannot be re-typed.
	rep.Snapshot = fs.captureSnapshot(dir, rep.URL,
		strings.TrimSpace(r.FormValue("table")), strings.TrimSpace(r.FormValue("seat")))

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

// captureSnapshot snapshots the table a report names, writes match.json,
// log.json and view.json beside it, and returns the status string for
// report.json's snapshot field. Every failure mode degrades to a reason
// recorded in that field — the report itself is never lost to a snapshot
// problem — and a report that names no table at all records why no snapshot
// was attempted.
func (fs *feedbackStore) captureSnapshot(dir, reportURL, tableField, seatField string) (status string) {
	tableID := tableField
	if tableID == "" {
		tableID = tableIDFromURL(reportURL)
	}
	if tableID == "" {
		return "unavailable: no table named"
	}
	if fs.reg == nil {
		return "unavailable: server has no table registry"
	}
	seat, _, err := feedbackSeat(seatField, reportURL)
	if err != nil {
		return "unavailable: " + err.Error()
	}
	defer func() {
		if p := recover(); p != nil {
			status = fmt.Sprintf("unavailable: snapshot panicked: %v", p)
		}
	}()
	snap, err := fs.reg.SnapshotForFeedback(host.TableID(tableID), seat)
	if err != nil {
		return "unavailable: " + err.Error()
	}

	matchJSON, err := json.MarshalIndent(snap.Match, "", "  ")
	if err != nil {
		return "unavailable: match.json: " + err.Error()
	}
	logJSON, err := json.MarshalIndent(snap.Log, "", "  ")
	if err != nil {
		return "unavailable: log.json: " + err.Error()
	}
	total := len(matchJSON) + len(logJSON)
	var viewJSON []byte
	if snap.View != nil {
		viewJSON, err = json.MarshalIndent(snap.View, "", "  ")
		if err != nil {
			return "unavailable: view.json: " + err.Error()
		}
		total += len(viewJSON)
	}
	if total > maxFeedbackSnapshotBytes {
		return fmt.Sprintf("unavailable: snapshot is %d bytes, over the %d-byte cap", total, maxFeedbackSnapshotBytes)
	}

	// The files are written best-effort too: a write failure records the
	// reason, and what did land stays. report.json (written after this
	// returns) is the durable status.
	var captured []string
	for _, f := range []struct {
		name string
		body []byte
	}{{"match.json", matchJSON}, {"log.json", logJSON}, {"view.json", viewJSON}} {
		if f.body == nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, f.name), append(f.body, '\n'), 0o644); err != nil {
			if len(captured) == 0 {
				return "unavailable: " + f.name + ": " + err.Error()
			}
			return fmt.Sprintf("partial: %s: %v (captured: %s)", f.name, err, strings.Join(captured, " "))
		}
		captured = append(captured, f.name)
	}
	if snap.View == nil && snap.ViewErr != "" {
		return fmt.Sprintf("partial: view unavailable: %s (captured: %s)", snap.ViewErr, strings.Join(captured, " "))
	}
	return "captured: " + strings.Join(captured, " ")
}

// feedbackSeat resolves the reporting seat: the explicit form field first,
// then the seat= query parameter of the report's own url (the live table
// route carries it today). hasSeat is false when neither names one — a
// valid shape, meaning "no seat's view was asked for".
func feedbackSeat(field, reportURL string) (*state.PlayerID, bool, error) {
	raw := field
	if raw == "" {
		if u, err := url.Parse(reportURL); err == nil {
			raw = u.Query().Get("seat")
		}
	}
	if raw == "" {
		return nil, false, nil
	}
	n, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return nil, false, fmt.Errorf("seat %q is not a seat number", raw)
	}
	p := state.PlayerID(n)
	return &p, true, nil
}

// tableIDFromURL extracts the table id a live-table URL names: the whole
// path segment after the /t/ route — "/t/t1?seat=0" (the live route) and
// "/t/t1/m/1" (an archived match's route, which the web router serves) both
// read t1. Splitting on path segments means only a complete segment counts,
// so a longer or lookalike path can never bleed into the id, and a "/t/"
// that appears only in the query string or fragment is not a route. The
// segment is percent-decoded, so a client that escaped the id
// ("/t/t1%2Fx") still reads the id the server registered; a segment that
// does not decode reads "" rather than a mangled id. Anything else (lobby,
// empty, unparseable) reads "".
func tableIDFromURL(reportURL string) string {
	u, err := url.Parse(strings.TrimSpace(reportURL))
	if err != nil {
		return ""
	}
	segs := strings.Split(u.Path, "/")
	for i, seg := range segs {
		if seg != "t" || i+1 >= len(segs) {
			continue
		}
		id, err := url.PathUnescape(segs[i+1])
		if err != nil {
			return ""
		}
		return id
	}
	return ""
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
