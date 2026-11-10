package kgen

import (
	"strings"
	"testing"
)

// helmScript is Worldwalker Helm's replacement line with the part a variant
// changes spliced in (the card's own text is not copied: this is the shape).
func helmScript(repl, body string) string {
	return "Name:Helm Probe\nManaCost:2 U\nTypes:Artifact\n" + repl + "\nSVar:DBReplace:" + body + "\n"
}

const (
	helmRepl = "R:Event$ CreateToken | ActiveZones$ Battlefield | ValidPlayer$ You | ValidToken$ Artifact | ReplaceWith$ DBReplace | Description$ x"
	helmBody = "DB$ ReplaceToken | Type$ AddToken | Amount$ 1 | TokenScript$ c_a_map_sac_explore"
)

// TestLowerReplCreatetokenValidplayer: a CreateToken replacement with
// ValidPlayer$ You lowers (Worldwalker Helm: the AddToken shape; the Map rides
// the card's token list), any other ValidPlayer and every out-of-shape adder
// still refuse by name.
func TestLowerReplCreatetokenValidplayer(t *testing.T) {
	helm := lowerCorpusCard(t, "Worldwalker Helm")
	for _, rf := range helm.Refusals {
		t.Errorf("Worldwalker Helm refused: %s", rf.Reason)
	}
	var add *lRepl
	for i := range helm.Repls {
		if helm.Repls[i].Kind == "AddToken" {
			add = &helm.Repls[i]
		}
	}
	if add == nil || add.S != "c_a_map_sac_explore" || add.F == nil {
		t.Fatalf("Worldwalker Helm lowered %+v, want an AddToken of the Map with a token filter", helm.Repls)
	}
	if len(helm.Repls) != 1 {
		t.Errorf("Worldwalker Helm lowered %d replacements, want 1", len(helm.Repls))
	}

	for _, c := range []struct{ name, repl, body, want string }{
		{"the shape", helmRepl, helmBody, ""},
		{"opponent's creations", strings.Replace(helmRepl, "ValidPlayer$ You", "ValidPlayer$ Opponent", 1), helmBody, "CreateToken ValidPlayer"},
		{"any player", strings.Replace(helmRepl, "ValidPlayer$ You", "ValidPlayer$ Player", 1), helmBody, "CreateToken ValidPlayer"},
		{"no player", strings.Replace(helmRepl, "ValidPlayer$ You | ", "", 1), helmBody, "AddToken without ValidPlayer$ You"},
		{"optional", strings.Replace(helmRepl, "ValidToken$", "Optional$ True | ValidToken$", 1), helmBody, "CreateToken Optional"},
		{"layer", strings.Replace(helmRepl, "ValidToken$", "Layer$ Copy | ValidToken$", 1), helmBody, "CreateToken Layer"},
		{"amount 2", helmRepl, strings.Replace(helmBody, "Amount$ 1", "Amount$ 2", 1), "CreateToken adder shape"},
		{"two scripts", helmRepl, strings.Replace(helmBody, "c_a_map_sac_explore", "c_a_map_sac_explore,c_a_treasure_sac_mana", 1), "CreateToken adder TokenScript"},
		{"chosen script", helmRepl, strings.Replace(helmBody, "c_a_map_sac_explore", "Chosen", 1), "CreateToken adder TokenScript"},
		{"opponent's tokens", strings.Replace(helmRepl, "ValidToken$ Artifact", "ValidToken$ Artifact.OppCtrl", 1), helmBody, "CreateToken adder ValidToken"},
		{"a rider on the body", helmRepl, helmBody + " | RememberTokens$ True", "CreateToken adder shape"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(lowerScript(t, "Helm Probe", helmScript(c.repl, c.body)), "; ")
			if c.want == "" {
				if got != "" {
					t.Fatalf("refused: %s", got)
				}
				return
			}
			if !strings.Contains(got, c.want) {
				t.Fatalf("refusals %q, want one naming %q", got, c.want)
			}
		})
	}
}
