package cardsettest

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// EDHPodSeats is a Commander pod's seat count.
const EDHPodSeats = 4

// EDHPod is one four-seat Commander pod on the edh pool: its table and each
// seat's 100 card ids.
type EDHPod struct {
	Table *kop.Table
	Decks [EDHPodSeats][]kop.CardID
}

// EDHPods builds n four-seat pods from the committed edh deck slice at
// slicePath, with dirac's BenchmarkTransitionEDH4 recipe: candidate c is the
// decks c, c+7, c+14 and c+21 of the name-sorted slice (wrapping), on the edh
// pool as kfuzz -allow-unverified plays it, and a candidate the kernel refuses
// for a card moves to the next. The pods are fixed by the slice and the tables
// alone. With no corpus or tables it skips, as the oracle's edh tests do.
func EDHPods(tb testing.TB, slicePath string, n int) []EDHPod {
	tb.Helper()
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		tb.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	decks := edhSliceDecks(tb, slicePath)
	out := make([]EDHPod, 0, n)
	for c := 0; len(out) < n; c++ {
		if c >= len(decks) {
			tb.Fatalf("only %d of %d edh pods build", len(out), n)
		}
		pick := make([][]cardset.Entry, EDHPodSeats)
		for j := range pick {
			pick[j] = decks[(c+7*j)%len(decks)]
		}
		tbl, ids, err := set.MatchupN(pick)
		if edhPodRefused(err) {
			continue
		}
		if err != nil {
			tb.Fatal(err)
		}
		p := EDHPod{Table: tbl}
		copy(p.Decks[:], ids)
		out = append(out, p)
	}
	return out
}

// edhSliceDeck is one deck file of the slice.
type edhSliceDeck struct {
	Cards []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"cards"`
}

// edhSliceDecks returns the slice's decks, 100 cards each, sorted by file name.
func edhSliceDecks(tb testing.TB, path string) [][]cardset.Entry {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	byName := map[string][]cardset.Entry{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			tb.Fatal(err)
		}
		if !strings.HasPrefix(h.Name, "decks/") || !strings.HasSuffix(h.Name, ".json") {
			continue
		}
		var d edhSliceDeck
		if err := json.NewDecoder(tr).Decode(&d); err != nil {
			tb.Fatalf("%s: %v", h.Name, err)
		}
		var es []cardset.Entry
		size := 0
		for _, c := range d.Cards {
			es = append(es, cardset.Entry{Name: c.Name, Count: c.Count})
			size += c.Count
		}
		if size != 100 {
			tb.Fatalf("%s: %d cards, want 100", h.Name, size)
		}
		byName[h.Name] = es
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([][]cardset.Entry, len(names))
	for i, n := range names {
		out[i] = byName[n]
	}
	if len(out) < EDHPodSeats {
		tb.Fatalf("%d edh decks, a pod needs %d", len(out), EDHPodSeats)
	}
	return out
}

// edhPodRefused is a refusal by name or by a card interaction the kernel
// guards against: that pod is skipped, any other error fails the caller.
func edhPodRefused(err error) bool {
	var re *cardset.RefusedError
	var ir *cardset.InteractionRefusal
	if errors.As(err, &re) || errors.As(err, &ir) {
		return true
	}
	if err == nil {
		return false
	}
	for _, p := range []string{"layer guard: ", "Room name read: ", "ReduceCost Type Ability beside "} {
		if strings.HasPrefix(err.Error(), p) {
			return true
		}
	}
	return false
}
