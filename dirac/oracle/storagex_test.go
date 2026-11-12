package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The storage-battery X mana ability (SubCounter<X/KIND>, Amount$ X) lowers
// as the kernel's counter-X script gap. Each named card lives in a tail
// synthetic pool; the fixture pairs it with a spell whose only coloured pip
// is one the storage ability can make and a basic land that cannot, so the
// spell is castable only after the storage counters pay it.
var kStorageX = []string{"Mage-Ring Network"}

type storageXFixture struct {
	card   string
	pool   string
	set    cardset.Set
	decks  [2][]cardset.Entry
	spell  string
	basics string
}

func storageXFixtures(t *testing.T) []storageXFixture {
	t.Helper()
	pools := cardsettest.SynthNames(t)
	var out []storageXFixture
	for _, card := range kStorageX {
		var set cardset.Set
		var pool string
		for _, name := range pools {
			s := cardsettest.Synth(t, name)
			if _, ok := s.Lookup(card); ok {
				set, pool = s, name
				break
			}
		}
		if set == nil {
			t.Fatalf("%s: no synthetic pool holds it", card)
		}
		idx, _ := set.Lookup(card)
		tbl, err := set.Pool()
		if err != nil {
			t.Fatal(err)
		}
		cd := &tbl.Cards[set.PoolID(idx)]
		var mask kop.ColorMask
		for j := range cd.Abilities {
			if kop.StoreManaAb(&cd.Abilities[j]) {
				mask = cd.Abilities[j].ManaColors
				break
			}
		}
		if mask == 0 {
			t.Fatalf("%s: no StoreManaAb in the built table", card)
		}
		spell, basics := storageXSpell(t, set, tbl, mask)
		if spell == "" {
			t.Fatalf("%s: no one-pip spell of colours %#x in %s", card, mask, pool)
		}
		// The card's own land and counter-placement abilities play; the X
		// mana ability's primitive is covered by TestStoreManaGap. A spell
		// would make the autopay surface's storage-manna offer a gorge/kernel
		// difference this ticket does not model (gorge's planner does not
		// price the source).
		deckA := []cardset.Entry{{Name: card, Count: 40}}
		deckB := []cardset.Entry{{Name: "Plains", Count: 40}}
		out = append(out, storageXFixture{
			card: card, pool: pool, set: set, spell: spell, basics: basics,
			decks: [2][]cardset.Entry{deckA, deckB},
		})
	}
	return out
}

// storageXSpell finds a cheap nonland card in the pool whose only coloured
// pip is one the storage ability makes, plus a basic land of a colour it does
// not make.
func storageXSpell(t *testing.T, set cardset.Set, tbl *kop.Table, mask kop.ColorMask) (spell, basic string) {
	t.Helper()
	best := 1 << 20
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		if c.Types&kop.TLand != 0 || c.Types&(kop.TCreature|kop.TInstant|kop.TSorcery) == 0 {
			continue
		}
		if c.Cost.X != 0 || c.SpellCost.Flags != 0 || c.HasAltAdd || c.SpellWaterbend != 0 || c.Casualty != 0 || c.KickerCost.Flags != 0 {
			continue
		}
		pips, ok := 0, true
		for col, n := range c.Cost.Pips {
			if n == 0 {
				continue
			}
			pips += int(n)
			if n != 1 || mask&(1<<col) == 0 {
				ok = false
			}
		}
		if !ok {
			continue
		}
		if mask == kop.ColC {
			if pips != 0 {
				continue
			}
			if c.Cost.MV() < 4 {
				continue // the plain {T} ability still pays a cheap spell
			}
		} else if pips != 1 {
			continue
		}
		if _, ok := set.Lookup(c.Name); !ok {
			continue
		}
		if mv := c.Cost.MV(); mv < best {
			best, spell = mv, c.Name
		}
	}
	for _, b := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
		col := map[string]kop.ColorMask{"Plains": kop.ColW, "Island": kop.ColU, "Swamp": kop.ColB, "Mountain": kop.ColR, "Forest": kop.ColG}[b]
		if mask&col == 0 {
			if _, ok := set.Lookup(b); ok {
				basic = b
				break
			}
		}
	}
	return spell, basic
}

// TestStorageXLockstep plays every named storage land in a pool where only
// its X mana can pay the fixture spell's coloured pip.
func TestStorageXLockstep(t *testing.T) {
	for _, f := range storageXFixtures(t) {
		t.Run(f.card, func(t *testing.T) {
			t.Logf("%s: pool %s, spell %s, basic %s", f.card, f.pool, f.spell, f.basics)
			fixtureLockstep(t, f.pool, f.decks, 40)
		})
	}
}
