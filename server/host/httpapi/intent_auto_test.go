package httpapi

// The ?auto=1 query form: the intent route forwards the client-autopilot
// flag into Registry.SubmitIntentFrom, which records it as bit 1
// (host.provAuto, value 2) of the match's provenance ledger. The caretaker
// bit (bit 0, value 1) must stay clear in this harness — its ThinkTimeout is
// 0, so no caretaker can ever fire. The bits are host-side metadata of the
// same kind as the snapshots: they never reach decision.Intent, so a
// request with and without ?auto=1 leaves the log identical (the
// host-package ledger tests pin that; here the handler's parse is what is
// under test).

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
)

// TestIntentAutoFlagForwarded POSTs one intent with ?auto=1 and the next
// without, then reads the registry's exported Provenance: the first
// accepted intent carries bit 1, the second does not, and no entry anywhere
// carries bit 0. The intent that follows a park is the first entry the
// ledger appends after the park (the engine is blocked on the parked
// decision while the client is answering), so index len(A) after the first
// POST is that intent — the assertion does not need host internals.
func TestIntentAutoFlagForwarded(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, _ := parkedSeat(t, r)
	ps := "s" + strconv.Itoa(int(parked))
	url := srv.URL + "/api/tables/t1/matches/1/intent"

	before, err := r.Provenance("t1", 1)
	if err != nil {
		t.Fatalf("Provenance before any submit: %v", err)
	}
	for _, b := range before {
		if b != 0 {
			t.Fatalf("entry %d is marked %d before any human submit", len(before), b)
		}
	}

	d, err := r.Pending("t1", 1, parked)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if code, e, _ := seatReq(t, http.MethodPost, url+"?auto=1", ps, answerFor(d)); code != http.StatusNoContent {
		t.Fatalf("auto intent: %d %+v, want 204", code, e)
	}
	autoBoundary := waitLedgerPast(t, r, len(before))

	// Logging the intent and posing the parked seat's next decision are not
	// the same moment: under load the host goroutine may have logged the
	// answer without having parked the seat again yet, so a single Pending
	// read can return "no decision pending". Poll until the next decision is
	// up (waitHTTPPending, the shape undo_test.go uses for the same race).
	d2 := waitHTTPPending(t, r, parked, d.Seq)
	// While d2 is parked the engine is blocked on it, so nothing appends:
	// the plain intent's entry is exactly the current length.
	mid, err := r.Provenance("t1", 1)
	if err != nil {
		t.Fatalf("Provenance before the plain submit: %v", err)
	}
	plainBoundary := len(mid)
	if code, e, _ := seatReq(t, http.MethodPost, url, ps, answerFor(d2)); code != http.StatusNoContent {
		t.Fatalf("plain intent: %d %+v, want 204", code, e)
	}
	waitLedgerPast(t, r, plainBoundary)

	prov, err := r.Provenance("t1", 1)
	if err != nil {
		t.Fatalf("Provenance: %v", err)
	}
	if got := prov[autoBoundary]; got&2 == 0 || got&1 != 0 {
		t.Fatalf("the ?auto=1 intent's entry is %d, want bit 1 set (auto) and bit 0 clear (no caretaker)", got)
	}
	if got := prov[plainBoundary]; got != 0 {
		t.Fatalf("the plain intent's entry is %d, want 0 (no flag, no caretaker)", got)
	}
	for i, b := range prov {
		if b&1 != 0 {
			t.Fatalf("entry %d carries the caretaker bit; this harness never arms a caretaker", i)
		}
	}
}

// waitLedgerPast polls Provenance until the ledger holds more than n
// entries — the accepted intent has logged — and fails after a deadline.
func waitLedgerPast(t *testing.T, r *host.Registry, n int) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		prov, err := r.Provenance("t1", 1)
		if err != nil {
			t.Fatalf("Provenance: %v", err)
		}
		if len(prov) > n {
			return n
		}
		if time.Now().After(deadline) {
			t.Fatalf("the accepted intent never logged: ledger stuck at %d entries", len(prov))
		}
		time.Sleep(time.Millisecond)
	}
}
