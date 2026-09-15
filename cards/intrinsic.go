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
	// Every path that constructs a Face (ParseBytes, the gob decode) runs
	// derive; this is the LAST load-time step that can change a face's
	// contents (it adds the intrinsic mana abilities below), so the derived
	// fields are refreshed here for EVERY face, not only the lands whose
	// abilities this call extends — a non-land face's derived values must
	// still equal what the gob decode route (which re-derives after decode,
	// over the same final content) computes. Deriving only inside the land
	// branch left every non-land face on the parse route holding the
	// pre-Link identity (the rv2c route-diff defect: Clay Champion, Dredging
	// Claw, Lashwrithe, Veteran's Powerblade).
	defer f.derive()
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
