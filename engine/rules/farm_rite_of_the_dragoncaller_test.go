package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window class 3way:FX|G (FDN-s508 xe:RAND:2:15:w18, FDN-s514
// xe:RAND:2:9:w18, field `permanents`, Rite of the Dragoncaller attributed
// but only a bystander): the window sets up p0's Enigma Drake with BOTH
// seats' Confiscates attached, p0's placed first and p1's second. Forge and
// XMage print "c1 o0 Enigma Drake"; gorge printed "c0 o0". CR 613.7: within
// layer 2 the later effect wins, so p1's Confiscate (the later timestamp)
// gives p1 control. gorge's static-control reconcile skipped p0's Confiscate
// when it began (p0 already controlled the Drake) without tracking it, so on
// the next reconcile, after p1's Confiscate had moved the Drake, p0's want
// looked new and stole the Drake back.
func TestFarmRiteoftheDragoncaller(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-rite-of-the-dragoncaller-two-confiscates",
	  "setup": {
	    "p0": {"battlefield": ["Enigma Drake", "Confiscate", "Island"], "attached": [{"card": "Confiscate", "to": "p0:Enigma Drake"}]},
	    "p1": {"battlefield": ["Confiscate", "Rite of the Dragoncaller", "Island"], "attached": [{"card": "Confiscate", "to": "p0:Enigma Drake"}]}
	  },
	  "steps": [{"op": "pass", "seat": 0}]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	if len(res.Snapshots) == 0 || res.Snapshots[0].Checkpoint != "setup" {
		t.Fatalf("no setup checkpoint: %+v", res.Snapshots)
	}
	for _, s := range res.Snapshots {
		auras := map[string]int{}
		var drake *OracleSnapPerm
		for i, p := range s.Permanents {
			switch p.Ref {
			case "p0:Confiscate", "p1:Confiscate":
				auras[p.Ref] = p.Controller
				// Precondition: both Auras are on the Drake, each under its
				// own seat, so the two layer-2 effects name different players.
				if p.AttachedTo != "p0:Enigma Drake" {
					t.Fatalf("%s: %s is attached to %q, want p0:Enigma Drake", s.Checkpoint, p.Ref, p.AttachedTo)
				}
			case "p0:Enigma Drake":
				drake = &s.Permanents[i]
			}
		}
		if auras["p0:Confiscate"] != 0 || auras["p1:Confiscate"] != 1 || len(auras) != 2 {
			t.Fatalf("%s: Confiscate controllers %v, want p0's under 0 and p1's under 1", s.Checkpoint, auras)
		}
		if drake == nil {
			t.Fatalf("%s: Enigma Drake is not on the battlefield", s.Checkpoint)
		}
		if drake.Owner != 0 || drake.Controller != 1 {
			t.Fatalf("%s: Enigma Drake is c%d o%d, want c1 o0 (p1's later Confiscate wins layer 2, CR 613.7)", s.Checkpoint, drake.Controller, drake.Owner)
		}
	}
}
