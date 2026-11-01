package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwobridRefusals: a twobrid symbol ({2/W}, CR 107.4e) is read only in
// a single-face card's printed mana cost; every other shape keeps or gains a
// named refusal.
func TestTwobridRefusals(t *testing.T) {
	for _, s := range []string{"2W", "R 2G", "2W 2U 2B"} {
		if _, err := parseManaCost(s); err == nil || !strings.Contains(err.Error(), "mana symbol") {
			t.Errorf("parseManaCost(%q): %v, want the mana symbol refusal (keyword and activation costs)", s, err)
		}
	}
	for _, s := range []string{"3W", "2C", "2/W", "W/2", "2WU", "22"} {
		if _, err := parsePrintedManaCost(s); err == nil && s != "22" {
			t.Errorf("parsePrintedManaCost(%q) accepted", s)
		}
	}
	m, err := parsePrintedManaCost("1 2U 2R")
	if err != nil || m.Generic != 1 || string(m.Twobrid) != "UR" {
		t.Fatalf("1 2U 2R: %+v %v", m, err)
	}
	if got := m.String(); got != "{1(2U)(2R)}" {
		t.Errorf("String %q", got)
	}
	kc, err := manaCostOf(m)
	if err != nil || kc.Hyb[0] != kop.HybTwobrid || kc.TbN() != 2 || kc.TbCol(0) != kop.U || kc.TbCol(1) != kop.R || kc.Generic != 1 {
		t.Fatalf("manaCostOf: %+v %v", kc, err)
	}
	for _, c := range []struct {
		cost, want string
	}{
		{"2W 2U 2B 2R 2G 2W", "twobrid pips 6"},
		{"2W W/U", "twobrid beside hybrid"},
		{"X 2W", "twobrid with {X}"},
	} {
		mc, err := parsePrintedManaCost(c.cost)
		if err != nil {
			continue // W/U is not a Forge hybrid spelling here; the check below builds it directly
		}
		if _, err := manaCostOf(mc); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.cost, err, c.want)
		}
	}
	h := manaCost{Twobrid: []byte("W"), Hybrid: []string{"BG"}}
	if _, err := manaCostOf(h); err == nil || err.Error() != "twobrid beside hybrid" {
		t.Errorf("twobrid beside hybrid: %v", err)
	}
	tb := manaCost{Twobrid: []byte("WWW")}
	for name, c := range map[string]*lCard{
		"kicker":    {Cost: tb, Kicker: &manaCost{Generic: 1}},
		"flashback": {Cost: tb, Flashback: &manaCost{Generic: 1}},
		"warp":      {Cost: tb, Warp: &manaCost{Generic: 1}},
		"plot":      {Cost: tb, Plot: &manaCost{Generic: 1}},
		"offspring": {Cost: tb, Offspring: &manaCost{Generic: 1}},
		"casualty":  {Cost: tb, Casualty: 1},
		"convoke":   {Cost: tb, Convoke: true},
		"improvise": {Cost: tb, Improvise: true},
		"altadd":    {Cost: tb, AltAddCost: &lCost{}},
		"spellcost": {Cost: tb, SpellCost: &lCost{}},
		"delve":     {Cost: tb, Kw: []string{"Delve"}},
		"face":      {Cost: tb, Faces: []string{"A", "B"}},
	} {
		if err := twobridConflict(c); err == nil {
			t.Errorf("%s: a twobrid cost beside it was accepted", name)
		}
	}
	if err := twobridConflict(&lCard{Cost: tb, Kw: []string{"Flying"}}); err != nil {
		t.Errorf("plain twobrid card refused: %v", err)
	}
}
