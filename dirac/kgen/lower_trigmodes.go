package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mode$ LandPlayed, SearchedLibrary, Forage and Discover.
//
// gorge trigmatch landPlayedMatches (actions.go): the land's move to the
// battlefield while it is a land play; Origin$ whitelists the zone played
// from (Ante and Command drop out), ValidCard$ is matched against the land on
// the battlefield. searchedLibraryMatches / forageMatches / discoverMatches:
// ValidPlayer$ against the acting player; SearchOwnLibrary$ True requires the
// searcher to be the library's owner. Nothing else is read, so any other key
// refuses.

// landOriginBits maps an Origin$ word of a LandPlayed line to its kop.Zone
// bit; ok is false for a word the kernel does not model, drop for the words
// gorge drops (a land is never played from the Ante or the Command zone).
func landOriginBits(w string) (bit int16, drop, ok bool) {
	switch w {
	case "Hand":
		return 1 << kop.ZoneHand, false, true
	case "Graveyard":
		return 1 << kop.ZoneGraveyard, false, true
	case "Exile":
		return 1 << kop.ZoneExile, false, true
	case "Library":
		return 1 << kop.ZoneLibrary, false, true
	case "Ante", "Command":
		return 0, true, true
	}
	return 0, false, false
}

// trigModes2 lowers the four modes. LandPlayed: ValidCard$ is F, Origin$ the
// zone mask N (0 = any). The others: ValidPlayer$ is Whose.
func (lc *lowerCtx) trigModes2(tp *params, tr *lTrigger, mode, zone string, readF func(string) (*lFilter, error)) error {
	if zone != "" {
		return refuse("%s trigger from an Effect", mode)
	}
	if err := trigExplEvZone(tr, mode); err != nil {
		return err
	}
	for _, k := range sortedParamKeys(tp.m) {
		switch k {
		case "Mode", "Execute", "TriggerZones", "TriggerDescription", "OptionalDecider":
		case "ValidCard", "Origin", "Secondary":
			if mode != "LandPlayed" {
				return refuse("%s %s$", mode, k)
			}
		case "ValidPlayer":
			if mode == "LandPlayed" {
				return refuse("%s %s$", mode, k)
			}
		case "ActivationLimit":
			if mode != "Discover" {
				return refuse("%s %s$", mode, k)
			}
		case "SearchOwnLibrary":
			if mode != "SearchedLibrary" {
				return refuse("%s %s$", mode, k)
			}
		default:
			return refuse("%s %s$", mode, k)
		}
	}
	if mode == "LandPlayed" {
		var err error
		if tr.F, err = readF("ValidCard"); err != nil {
			return err
		}
		if v, ok := tp.get("Origin"); ok {
			var mask int16
			for _, w := range strings.Split(v, ",") {
				bit, drop, ok := landOriginBits(strings.TrimSpace(w))
				if !ok {
					return refuse("LandPlayed Origin$ %q", v)
				}
				if !drop {
					mask |= bit
				}
			}
			if mask == 0 {
				return refuse("LandPlayed Origin$ %q", v)
			}
			tr.N = mask
		}
		tr.Event = mode
		return nil
	}
	if mode == "SearchedLibrary" && tp.str("SearchOwnLibrary") != "True" {
		return refuse("SearchedLibrary without SearchOwnLibrary$ True")
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("%s ValidPlayer %q", mode, tp.str("ValidPlayer"))
	}
	tr.Whose, tr.Event = w, mode
	return nil
}
