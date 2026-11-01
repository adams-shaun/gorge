package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestRoomLKIAbility pins the Room source-card change branch of
// realSide.Project: a Room's door state is another def of the same printed
// face (CR 709.5), so gorge's abilityIndex still finds the queued SA in the
// unchanged src.Face(). A source that really flipped stays 255.
func TestRoomLKIAbility(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fullyunlock")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(kfullyDecks[0], kfullyDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfullyDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	m, err := NewMatchup(reg, [2]string{"kroomlki-a", "kroomlki-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	side, err := SetSide(set, tbl, Config{})(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(side)
	rs, ok := side.(*realSide)
	if !ok {
		t.Fatalf("SetSide returned %T, want *realSide", side)
	}
	find := func(name string) dirac.CardID {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == name {
				return dirac.CardID(i)
			}
		}
		t.Fatalf("%q not in table %s", name, tbl.Name)
		return 0
	}

	plain := find("Defiled Crypt")         // cast face; ability 0 is its leaves trigger (origin 128)
	state := find("room:Defiled Crypt#03") // same printed face, both doors unlocked
	other := find("room:Meat Locker#03")   // a different Room's state def

	if got := rs.lkiAbility(state, plain, 0); got != 128 {
		t.Errorf("lkiAbility(room:Defiled Crypt#03, Defiled Crypt, 0) = %d, want 128", got)
	}
	if got := rs.lkiAbility(other, plain, 0); got != 255 {
		t.Errorf("lkiAbility(room:Meat Locker#03, Defiled Crypt, 0) = %d, want 255", got)
	}
	if got := rs.lkiAbility(plain, plain, 0); got != 128 {
		t.Errorf("lkiAbility(Defiled Crypt, Defiled Crypt, 0) = %d, want 128", got)
	}
	if got := rs.lkiAbility(state, state, 1); got != 255 {
		t.Errorf("lkiAbility(room:Defiled Crypt#03, room:Defiled Crypt#03, 1) = %d, want 255", got)
	}
	// The same assertions through the branch decision Project calls, so
	// putting st.Ability = 255 back in Project is caught here too.
	if got := rs.stackAbility(state, plain, 0); got != 128 {
		t.Errorf("stackAbility(room:Defiled Crypt#03, Defiled Crypt, 0) = %d, want 128", got)
	}
	if got := rs.stackAbility(other, plain, 0); got != 255 {
		t.Errorf("stackAbility(room:Meat Locker#03, Defiled Crypt, 0) = %d, want 255", got)
	}
	if got := rs.stackAbility(plain, plain, 0); got != 128 {
		t.Errorf("stackAbility(Defiled Crypt, Defiled Crypt, 0) = %d, want 128", got)
	}
	if got := rs.stackAbility(state, state, 1); got != 255 {
		t.Errorf("stackAbility(room:Defiled Crypt#03, room:Defiled Crypt#03, 1) = %d, want 255", got)
	}
}
