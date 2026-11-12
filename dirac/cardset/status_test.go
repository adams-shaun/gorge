package cardset

import (
	"errors"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// statusPool holds one card of each status and a token.
func statusPool() *PoolData {
	return &PoolData{Name: "st", NumCards: 4, NumTokens: 1, NumIDs: 5,
		Names:  []string{"A Good", "B Unlowered", "C Refused", "D Unproven", "T Token"},
		PoolID: []uint16{0, 1, 2, 3, 4},
		Status: []Status{StatusSupported, StatusNotLowered, StatusRefused, StatusUnverified, StatusSupported},
		Reason: []string{"", "not lowered: no", "refused: lockstep: it differs; catch-up pending", "unverified: in no evidence list", ""},
		Defs:   make([]kop.CardDef, 5), CardSubs: make([]kop.SubMask, 5), CardTokens: make([][]uint16, 5),
		CardRefSubs: make([][]uint8, 5), AbilityOrigin: make([][]uint8, 5), StaticSubs: make([][]AbSubs, 5),
		Filters: []kop.Filter{{}}, Exprs: []kop.Expr{}, Conds: []kop.Cond{}, Overrides: []kop.Override{}, Emblems: []kop.Emblem{}}
}

func matchupErr(s Set, card string) error {
	_, _, _, err := s.Matchup([]Entry{{Name: card, Count: 1}}, nil)
	return err
}

func TestMatchupRefusesByNameWithReason(t *testing.T) {
	d := statusPool()
	err := matchupErr(viewOf(d, gated(false)), "C Refused")
	var re *RefusedError
	if !errors.As(err, &re) {
		t.Fatalf("a refused card: %v, want a RefusedError", err)
	}
	if re.Card != "C Refused" || re.Status != StatusRefused || re.Reason != d.Reason[2] {
		t.Errorf("RefusedError %+v", re)
	}
	for _, want := range []string{`"C Refused"`, "it differs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not say %q", err, want)
		}
	}
}

// Each view against each status: what the views exist to keep apart.
func TestViewsPlayWhatTheySay(t *testing.T) {
	d := statusPool()
	views := []struct {
		name string
		set  Set
		want map[string]bool
	}{
		{"gated", viewOf(d, gated(false)), map[string]bool{"A Good": true, "D Unproven": true}},
		{"enforced", viewOf(d, gated(true)), map[string]bool{"A Good": true}},
		{"allow unverified", AllowUnverified(viewOf(d, gated(true))), map[string]bool{"A Good": true, "D Unproven": true}},
		{"raw", viewOf(d, lowered), map[string]bool{"A Good": true, "C Refused": true, "D Unproven": true}},
	}
	for _, v := range views {
		for i, n := range d.Names[:d.NumCards] {
			err := matchupErr(v.set, n)
			if (err == nil) != v.want[n] {
				t.Errorf("%s: %s: %v", v.name, n, err)
			}
			if v.set.Inert(i) == v.want[n] {
				t.Errorf("%s: %s: Inert %v", v.name, n, v.set.Inert(i))
			}
		}
	}
}

func TestUnverifiedCardIsRefusedWhenEnforced(t *testing.T) {
	d := statusPool()
	err := matchupErr(viewOf(d, gated(true)), "D Unproven")
	var re *RefusedError
	if !errors.As(err, &re) || re.Status != StatusUnverified {
		t.Fatalf("enforced: %v, want a RefusedError for an unverified card", err)
	}
	if err := matchupErr(viewOf(d, gated(false)), "D Unproven"); err != nil {
		t.Errorf("not enforced: %v", err)
	}
	if err := matchupErr(viewOf(d, gated(true)), "A Good"); err != nil {
		t.Errorf("a supported card, enforced: %v", err)
	}
}

// The zero Status reads as supported, so a table that predates statuses, or lost
// them, must not load as one where every card is.
func TestStatusZeroCannotLoad(t *testing.T) {
	b := EncodeTable(statusPool(), "k")
	if _, _, err := DecodeTable(b); err != nil {
		t.Fatal(err)
	}
	old := append([]byte(nil), b...)
	old[len(tableMagic)] = 1
	if _, _, err := DecodeTable(old); err == nil || !strings.Contains(err.Error(), "table format 1") {
		t.Errorf("an old format: %v", err)
	}
	none := statusPool()
	none.Status, none.Reason = nil, nil
	if _, _, err := DecodeTable(EncodeTable(none, "k")); err == nil || !strings.Contains(err.Error(), "0 statuses") {
		t.Errorf("a table with no statuses: %v", err)
	}
}

func TestSupportSlug(t *testing.T) {
	for card, want := range map[string]string{
		"Rockface Village":          "rockface-village",
		"Al-abara's Carpet":         "al-abara-s-carpet",
		"Fire // Ice":               "fire-ice",
		"Circle of Protection: Red": "circle-of-protection-red",
	} {
		if got := SupportSlug(card); got != want {
			t.Errorf("SupportSlug(%q) = %q, want %q", card, got, want)
		}
	}
}
