package canon

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestVoteCardCanon: the kernel's ballot ask is gorge's KChoose (DChoose) and
// each pick is the "vote_card" choose kind over the permanent voted for, as
// gorge effCardVote's options canonicalise (oracle/canon.go's default arm).
func TestVoteCardCanon(t *testing.T) {
	code, ok := ChooseCode("vote_card")
	if !ok {
		t.Fatal("vote_card is not in ChooseKinds")
	}
	s := new(dirac.State)
	d := &dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChVoteCard, Player: 1, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, 4, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkChoose, 9, 0, 0)
	tr := &Translator{S: s, D: d}
	if got := tr.DKind(); got != DChoose {
		t.Fatalf("decision kind %v, want DChoose", got)
	}
	for i, src := range []Ref{4, 9} {
		got := tr.Action(d.Options[i])
		want := Pack(ActChoose+CanonKind(code), src, RefNone, 0)
		if got != want {
			t.Fatalf("option %d = %#x (kind %d src %d tgt %d mode %d), want %#x", i,
				got, got.Kind(), got.Src(), got.Tgt(), got.Mode(), want)
		}
	}
}
