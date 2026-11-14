package cardset

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The guards name the cards they refuse, though a table's Defs carry no names.
func TestGuardMessagesNameTheCards(t *testing.T) {
	entries := []Entry{{Name: "A Card", Count: 1}, {Name: "B Card", Count: 1}}
	data := func() *PoolData {
		return &PoolData{
			Name: "t", NumCards: 2, NumIDs: 2,
			Names:       []string{"A Card", "B Card"},
			PoolID:      []uint16{0, 1},
			Status:      make([]Status, 2),
			Reason:      make([]string, 2),
			Defs:        make([]kop.CardDef, 2),
			CardSubs:    make([]PoolSubMask, 2),
			CardTokens:  make([][]uint16, 2),
			CardRefSubs: make([][]uint8, 2),
			StaticSubs:  make([][]AbSubs, 2),
		}
	}
	for _, c := range []struct {
		name string
		set  func(d *PoolData)
		want string
	}{
		{"layer", func(d *PoolData) {
			d.LayerCls = []LayerEntry{{I: 0, C: LayerClasses{Any: LcRecPT, Other: LcRecPT}}, {I: 1, C: LayerClasses{Any: LcStatPT, Other: LcStatPT}}}
			d.LayerConflicts = []LayerConflict{{A: LcRecPT, B: LcStatPT, Why: "they conflict"}}
		}, `"A Card" and "B Card": they conflict`},
		{"room", func(d *PoolData) { d.RoomCards, d.NameReaders = []int{0}, []int{1} }, "B Card reads a name and A Card is a Room"},
		{"ab reduce", func(d *PoolData) {
			d.AbReducers, d.AbReducerFloor, d.AbReducerTypes, d.AbManaCosted, d.AbManaCostedMV, d.AbManaCostedTypes = []int{0}, []uint8{0}, []uint16{0}, []int{1}, []uint8{1}, []uint16{2}
		}, "A Card reduces abilities and B Card has a mana ability"},
		{"ab reduce floor below the cost", func(d *PoolData) {
			d.AbReducers, d.AbReducerFloor, d.AbReducerTypes, d.AbManaCosted, d.AbManaCostedMV, d.AbManaCostedTypes = []int{0}, []uint8{1}, []uint16{2}, []int{1}, []uint8{2}, []uint16{2}
		}, "A Card reduces abilities and B Card has a mana ability"},
	} {
		d := data()
		c.set(d)
		_, _, _, err := NewDataSet(d).Matchup(entries, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	// A MinMana$ floor at the mana ability's cost leaves it as printed.
	d := data()
	d.AbReducers, d.AbReducerFloor, d.AbReducerTypes, d.AbManaCosted, d.AbManaCostedMV, d.AbManaCostedTypes = []int{0}, []uint8{1}, []uint16{0}, []int{1}, []uint8{1}, []uint16{2}
	if _, _, _, err := NewDataSet(d).Matchup(entries, nil); err != nil {
		t.Errorf("a floored reducer beside a one-mana ability: %v", err)
	}
	// A reducer whose filter names a type the costed card lacks lowers nothing.
	d = data()
	d.AbReducers, d.AbReducerFloor, d.AbReducerTypes, d.AbManaCosted, d.AbManaCostedMV, d.AbManaCostedTypes = []int{0}, []uint8{0}, []uint16{1}, []int{1}, []uint8{1}, []uint16{2}
	if _, _, _, err := NewDataSet(d).Matchup(entries, nil); err != nil {
		t.Errorf("a land-only reducer beside a creature's mana ability: %v", err)
	}
	d = data()
	d.Status[1], d.Reason[1] = StatusNotLowered, "not lowered: no"
	_, _, _, err := NewDataSet(d).Matchup(entries, nil)
	if err == nil || !strings.Contains(err.Error(), `"B Card" is not supported: not lowered: no`) {
		t.Errorf("unsupported card: %v", err)
	}
}
