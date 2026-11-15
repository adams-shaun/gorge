package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestOutsideGrantsDistinctCodes drives the walk's outside-grant
// accumulator: several AddChosen grants of outside types on one creature
// answer a ChosenType read exactly when every code is known, and stay
// loud only for aliased equal codes or more grants than the walk holds
// (CR 205.3m, 613.1d).
func TestOutsideGrantsDistinctCodes(t *testing.T) {
	sc, _ := newObjScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	if sc.k.gen == nil {
		t.Skip("FDN table carries no generated extension")
	}
	sc.k.gen.anyAddChosen = true

	// One outside code names exactly one type (unaliased); another names
	// several (aliased). They must differ, or the rows below compare
	// nothing.
	var plain, aliased uint8
	plainOK, aliasedOK := false, false
	for v, si := range sc.k.t.TypeVocab {
		if si != 255 {
			continue
		}
		ch := typeVocabSub(sc.k, v)
		if outsideAliased(sc.k, ch) {
			aliased, aliasedOK = ch, true
		} else {
			plain, plainOK = ch, true
		}
	}
	if !plainOK || !aliasedOK || plain == aliased {
		t.Fatalf("no distinct unaliased/aliased outside codes: %d/%v %d/%v", plain, plainOK, aliased, aliasedOK)
	}
	var third uint8
	for v, si := range sc.k.t.TypeVocab {
		if si != 255 {
			continue
		}
		if ch := typeVocabSub(sc.k, v); ch != plain && ch != aliased {
			third = ch
			break
		}
	}
	if third == plain || third == aliased {
		t.Fatalf("no third outside code besides %d and %d", plain, aliased)
	}
	var five []uint8
	for v, si := range sc.k.t.TypeVocab {
		if si != 255 {
			continue
		}
		if ch := typeVocabSub(sc.k, v); !slices.Contains(five, ch) {
			five = append(five, ch)
			if len(five) == outsMax+1 {
				break
			}
		}
	}
	if len(five) != outsMax+1 {
		t.Fatalf("%d distinct outside codes, want %d", len(five), outsMax+1)
	}

	grant := func(c *dchars, code uint8, src Slot) {
		t.Helper()
		lfxChosen(sc.k, &lfx{ch: code, src: src}, c)
	}
	for _, r := range []struct {
		name    string
		grants  []outGrant // code and source, in walk order
		askCode uint8
		askSrc  Slot
		wantN   uint8
		want    bool
		stall   uint8
	}{
		{
			name:    "ownGrantPresent",
			grants:  []outGrant{{plain, 1}, {aliased, 2}},
			askCode: plain, askSrc: 1, wantN: 2, want: true,
		},
		{
			name:    "differentCodesAbsent",
			grants:  []outGrant{{plain, 1}, {aliased, 2}},
			askCode: third, askSrc: 3, wantN: 2, want: false,
		},
		{
			name:    "aliasedEqualFromAnotherSource",
			grants:  []outGrant{{aliased, 1}, {aliased, 2}},
			askCode: aliased, askSrc: 3, wantN: 2, want: false, stall: StallOutsideType,
		},
		{
			name:    "tooManyGrants",
			grants:  []outGrant{{five[0], 1}, {five[1], 2}, {five[2], 3}, {five[3], 4}, {five[4], 5}},
			askCode: five[0], askSrc: 6, wantN: outsOver, want: false, stall: StallOutsideType,
		},
		{
			name:    "singleOwnGrant",
			grants:  []outGrant{{plain, 1}},
			askCode: plain, askSrc: 1, wantN: 1, want: true,
		},
	} {
		t.Run(r.name, func(t *testing.T) {
			var c dchars
			for _, g := range r.grants {
				grant(&c, g.code, Slot(g.src-1))
			}
			if c.nOuts != r.wantN {
				t.Fatalf("nOuts %d, want %d (grants %v)", c.nOuts, r.wantN, r.grants)
			}
			sc.s.perms[r.askSrc].chosen = objWord(r.askCode)
			sc.s.g.stalled = 0
			if got := sc.s.curOutsideHas(sc.k, &c, r.askSrc); got != r.want {
				t.Fatalf("curOutsideHas = %v, want %v", got, r.want)
			}
			if got := sc.s.g.stalled; got != r.stall {
				t.Fatalf("stall %d, want %d", got, r.stall)
			}
		})
	}
}
