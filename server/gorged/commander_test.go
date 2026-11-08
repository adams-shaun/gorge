package gorged

import "testing"

func TestCheckDiracFlagsCommander(t *testing.T) {
	c := diracConfig()
	c.formatsRaw = "constructed,commander"
	c.diracSets = "cmd5"
	if err := c.checkDiracFlags(); err != nil {
		t.Fatalf("-format commander with -engine dirac -seats 2: %v", err)
	}
	c.seats = 4
	if err := c.checkDiracFlags(); err == nil {
		t.Fatal("four seats passed the flag check")
	}
}
