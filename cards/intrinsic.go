package cards

// basicLandMana maps a land subtype to the mana it taps for. The corpus omits
// these abilities entirely: Forge's engine grants them from the subtype, so any
// port must supply the same layer.
var basicLandMana = []struct{ Subtype, Color string }{
	{"Plains", "W"}, {"Island", "U"}, {"Swamp", "B"},
	{"Mountain", "R"}, {"Forest", "G"}, {"Wastes", "C"},
}

// ApplyIntrinsics adds abilities the engine grants rather than the script.
// It is idempotent: calling it twice adds nothing the second time.
func (f *Face) ApplyIntrinsics() {
	if !f.IsLand() {
		return
	}
	have := map[string]bool{}
	for _, a := range f.ManaAbilities() {
		have[a.Params["Produced"]] = true
	}
	// Iterate the fixed slice, not a map, so ability order is deterministic.
	for _, b := range basicLandMana {
		if !f.hasType(b.Subtype) || have[b.Color] {
			continue
		}
		have[b.Color] = true
		f.Abilities = append(f.Abilities, &SA{
			Kind: "AB", API: "Mana",
			Params: map[string]string{"Cost": "T", "Produced": b.Color, "Amount": "1"},
			Line:   "intrinsic: basic land mana",
		})
	}
}
