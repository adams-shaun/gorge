package draftlab

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

func TestRiderSetsFromSpecs(t *testing.T) {
	if !isRider("SPG") {
		t.Error("isRider(SPG) = false, want true (FDN/BLB source_set_codes)")
	}
	if !isRider("SDS") {
		t.Error("isRider(SDS) = false, want true (TRK source_set_codes)")
	}
	for _, code := range setdata.Codes() {
		if isRider(code) {
			t.Errorf("isRider(%s) = true, want false (an embedded set's own code)", code)
		}
	}
	if isRider("") {
		t.Error(`isRider("") = true, want false`)
	}
}

func TestPackSetIgnoresRiders(t *testing.T) {
	trk := func(id, name string) limited.CardRef {
		return limited.CardRef{ID: id, Name: name, Set: "TRK"}
	}
	pod := func(cards ...[]limited.CardRef) [][][]limited.CardRef {
		// 4 seats, 1 round: every seat opens the one pack given.
		packs := make([][][]limited.CardRef, 4)
		for s := range packs {
			packs[s] = [][]limited.CardRef{cards[s]}
		}
		return packs
	}
	res, err := packSet(pod(
		[]limited.CardRef{trk("c1", "a")},
		[]limited.CardRef{{ID: "c2", Name: "b", Set: "SDS"}},
		[]limited.CardRef{trk("c3", "c")},
		[]limited.CardRef{trk("c4", "d")},
	))
	if err != nil {
		t.Fatalf("packSet: %v", err)
	}
	if res != "TRK" {
		t.Errorf("packSet = %q, want TRK", res)
	}

	_, err = packSet(pod(
		[]limited.CardRef{{ID: "c1", Name: "a", Set: "SDS"}},
		[]limited.CardRef{{ID: "c2", Name: "b", Set: "SDS"}},
		[]limited.CardRef{{ID: "c3", Name: "c", Set: "SDS"}},
		[]limited.CardRef{{ID: "c4", Name: "d", Set: "SDS"}},
	))
	if err == nil || !strings.Contains(err.Error(), "bonus-sheet cards (SDS)") {
		t.Errorf("rider-only pod error = %v, want bonus-sheet cards (SDS)", err)
	}

	_, err = packSet(pod(
		[]limited.CardRef{trk("c1", "a")},
		[]limited.CardRef{{ID: "c2", Name: "b", Set: "FDN"}},
		[]limited.CardRef{trk("c3", "c")},
		[]limited.CardRef{trk("c4", "d")},
	))
	if err == nil || !strings.Contains(err.Error(), "mixed sets") {
		t.Errorf("TRK+FDN pod error = %v, want mixed sets", err)
	}

	set, err := refsSet([]limited.CardRef{
		trk("c1", "a"),
		{ID: "c2", Name: "b", Set: "SDS"},
		trk("c3", "c"),
	})
	if err != nil {
		t.Fatalf("refsSet: %v", err)
	}
	if set != "TRK" {
		t.Errorf("refsSet = %q, want TRK", set)
	}
}

func TestDraftTRKPodWithStardates(t *testing.T) {
	l := testLab(t)
	var pod [][][]limited.CardRef
	var seed uint64
	for s := uint64(1000); s < 1050; s++ {
		p, err := l.OpenPod("TRK", setdata.Arena, s, 8, 3)
		if err != nil {
			t.Fatalf("OpenPod(TRK, seed %d): %v", s, err)
		}
		for _, rounds := range p {
			for _, pack := range rounds {
				for _, c := range pack {
					if c.Set == "SDS" {
						pod = p
						seed = s
					}
				}
			}
		}
		if pod != nil {
			break
		}
	}
	if pod == nil {
		t.Fatal("no TRK pod among seeds 1000..1049 carries an SDS card; the precondition the test exists for is gone")
	}
	res, err := l.Draft(pod, newDrafters(t, l, "rating-colour", 1), seed)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if res.Set != "TRK" {
		t.Errorf("Draft result set = %q, want TRK", res.Set)
	}
	for s, pool := range res.Pools {
		hasSDS := false
		for _, c := range pool {
			if c.Set == "SDS" {
				hasSDS = true
				break
			}
		}
		if !hasSDS {
			continue
		}
		deck, err := l.Build(pool, testBuilder(t, l, "best23"))
		if err != nil {
			t.Fatalf("seat %d Build: %v", s, err)
		}
		if deck.Set != "TRK" {
			t.Errorf("seat %d deck set = %q, want TRK", s, deck.Set)
		}
		break
	}
}
