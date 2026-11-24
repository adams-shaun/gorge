package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManaSpentInterveningIfForms: a printed self-ETB trigger whose own
// description reads "if {C} was spent to cast it" carries the gate on its
// Execute body for Forge, but CR 603.4 makes it an intervening-if. kgen adds
// the queue-time gate beside the body's live resolution read: the header
// vAdamant has no kop.AdamantLiveRead bit, while the body read keeps it. A
// "sacrifice it unless {C} was spent" body (Azorius Herald) is not an
// intervening-if and must not gain the header gate.
func TestManaSpentInterveningIfForms(t *testing.T) {
	cardsDir := ratchetCards(t)
	names := []string{"Ogre Savant", "Steamcore Weird", "Revenant Patriarch", "Azorius Herald"}
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	// Header gates: colour index | 1<<3, no AdamantLiveRead.
	header := map[string]string{
		"Ogre Savant":        "Adamant(n=9,1,0)",
		"Steamcore Weird":    "Adamant(n=11,1,0)",
		"Revenant Patriarch": "Adamant(n=8,1,0)",
	}
	for name, gate := range map[string]string{
		"Ogre Savant":        header["Ogre Savant"],
		"Steamcore Weird":    header["Steamcore Weird"],
		"Revenant Patriarch": header["Revenant Patriarch"],
	} {
		c := byName[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), gate) {
			t.Errorf("%s: lowered IR lacks the queue-time intervening-if %q:\n%s", name, gate, sb.String())
		}
	}
	az := byName["Azorius Herald"]
	if az == nil {
		t.Fatal("Azorius Herald: not lowered")
	}
	var sb strings.Builder
	dumpLIR(&sb, az)
	if strings.Contains(sb.String(), "Adamant(n=8,") {
		t.Errorf("Azorius Herald: 'unless {U} was spent' body gained a queue-time gate:\n%s", sb.String())
	}
	if !strings.Contains(sb.String(), "Adamant(n=41,1,0)") {
		t.Errorf("Azorius Herald: body's live ConditionManaNotSpent$ read missing:\n%s", sb.String())
	}
}
