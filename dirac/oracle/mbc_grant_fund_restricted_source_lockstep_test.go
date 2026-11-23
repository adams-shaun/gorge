package oracle

import "testing"

// CR 106.6: mana whose source carries "RestrictValid$
// CantCastNonArtifactSpells" may still pay anything but a nonartifact spell
// cast, so gorge's potential planner's coveredSpendable (potential_script.go)
// walks its tap as a funding source: a land's "{1}" activation fee admits it.
// The kernel's grant-search funding pass skipped it (its restricted printed
// mana is not tap-for-mana in the kernel's prep), walked three funders where
// gorge walked five, found the script inside the 160-node budget where gorge
// burned the nodes and died -- and offered the cast gorge dropped. gorge
// walked five (the Powerstone Tokens among them) and died at exactly node 160
// mid-funding-branch; the kernel found the line at 111 (dr-mbc-bbab pair 12,
// decision 299).
func TestMBCHazezonShaperOfSandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_fund_restricted_source")
}
