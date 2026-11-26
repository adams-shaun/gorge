package gorged

import (
	"context"
	"github.com/adams-shaun/gorge/testkit/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestArtBackgroundFillStopsWithTheCache pins serve's shutdown contract for
// the fills that outlive their caller: closeBackground cancels a
// queueTextFill still waiting on Scryfall and returns only once it has, and
// no fill starts after it. A fill that kept running past serve's return wrote
// into a test's TempDir during its RemoveAll ("directory not empty",
// TestDiracManaBrewLandUndoKeepsLandPlays).
func TestArtBackgroundFillStopsWithTheCache(t *testing.T) {
	entered := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done() // an upstream that never answers
	}))
	defer srv.Close()
	ac, err := newArtCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ac.client, ac.namedBaseURL, ac.pace = srv.Client(), srv.URL+"/cards/named?exact=", 0

	ac.queueTextFill("Grizzly Bears")
	select {
	case <-entered:
	case <-time.After(testutil.WaitBudget):
		t.Fatal("the queued fill never reached the upstream")
	}
	closed := make(chan struct{})
	go func() {
		ac.closeBackground()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(testutil.WaitBudget):
		t.Fatal("closeBackground did not cancel the fill waiting on the upstream")
	}
	if ac.background(func(_ context.Context) { t.Error("a fill ran after closeBackground") }) {
		t.Fatal("background accepted a fill after closeBackground")
	}
}
