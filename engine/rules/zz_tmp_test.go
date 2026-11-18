package rules

import "testing"

func TestTmpFeeVariants(t *testing.T) {
	base := []string{"Swamp", "Swamp", "Swamp", "Unknown Shores", "Hydraulic Helper"}
	for _, v := range []struct {
		name  string
		board []string
		card  string
	}{
		{"4island+oracle", []string{"Island", "Island", "Island", "Island"}, "Sea Gate Oracle"},
		{"base+strat", base, "Trickster's Stratagem"},
		{"base+oracle", base, "Sea Gate Oracle"},
		{"base+RH+strat", append(append([]string{}, base...), "Rewrite History"), "Trickster's Stratagem"},
		{"nohelper+strat", base[:4], "Trickster's Stratagem"},
		{"4swamp+strat", []string{"Swamp", "Swamp", "Swamp", "Swamp", "Swamp"}, "Trickster's Stratagem"},
		{"3swamp+island+strat", []string{"Swamp", "Swamp", "Swamp", "Island"}, "Trickster's Stratagem"},
	} {
		e := filterFeeBoard(t, v.board, []string{v.card})
		id := boundHandID(t, e, v.card)
		e.pending = nil
		e.Advance()
		d := e.Pending()
		t.Errorf("pending kind %v player %d step %v turn %d opts %d", d.Kind, d.Player, e.G.Step, e.G.Turn, len(d.Options))
		n := 0
		for _, o := range castOptions(t, e) {
			if o.Obj == id {
				n++
			}
		}
		t.Errorf("%s: offered %d (legalActions %d)", v.name, n, len(castOptionsFor(e, id)))
	}
}
