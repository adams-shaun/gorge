package kgen

// cmdGreatestMVSpec is the one Count$ValidBattlefield,Command spelling the
// corpus writes (Cloudkill, Majestic Genesis): "the greatest mana value of a
// commander you own on the battlefield or in the command zone".
const cmdGreatestMVSpec = "Card.IsCommander+YouOwn$GreatestCardManaCost"

// cmdGreatestMVValue lowers Count$ValidBattlefield,Command <spec> (gorge
// count_body_zone.go's multi-zone fold over multiCountZones' two zones,
// extremePropertyValue's objectManaValue) to kop.ExCmdGreatestMV. Only the
// commander mana-value spelling is read: the kernel has no general
// command-zone walk, and a seat's commanders are the only cards there that
// IsCommander matches (CR 903.3). Any other spec refuses.
func cmdGreatestMVValue(spec string) (lValue, error) {
	if spec != cmdGreatestMVSpec {
		return lValue{}, refuse("Count$ValidBattlefield,Command %q: only %s is read", spec, cmdGreatestMVSpec)
	}
	return lValue{Kind: vCmdGreatestMV}, nil
}

// cmdGreatestMVAll lowers Count$ValidAll <spec> (gorge count_body_zone.go's
// every-zone fold, each candidate matched in its own zone) for the one
// commander spelling (Tangleweave Armor: "the greatest mana value among your
// commanders"): kop.ExCmdGreatestMV over every zone (kop.CmdMVAllZones). A
// seat's commanders are the only cards IsCommander matches (CR 903.3), so
// the kernel reads them wherever they are. Any other spec refuses.
func cmdGreatestMVAll(spec string) (lValue, error) {
	if spec != cmdGreatestMVSpec {
		return lValue{}, refuse("Count$ValidAll %q: only %s is read", spec, cmdGreatestMVSpec)
	}
	return lValue{Kind: vCmdGreatestMV, N: 1}, nil
}
