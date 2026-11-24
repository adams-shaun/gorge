package oracle

// The life-lost extremes over a group of players (gorge playerCountExtreme's
// LifeLostThisTurn arm): the opponents, spelled Registered, and the Defined
// Registered group, every living player (the highest only) or with .Other the opponents. Each is
// ExOppExtreme's OppExtLifeLost figure, walked in seat order, so a pod's
// three opponents are read one by one.
func init() {
	for _, h := range []string{
		"PlayerCountRegisteredOpponents$HighestLifeLostThisTurn", "PlayerCountRegisteredOpponents$LowestLifeLostThisTurn",
		"PlayerCountDefinedRegistered$HighestLifeLostThisTurn",
		"PlayerCountDefinedRegistered.Other$HighestLifeLostThisTurn", "PlayerCountDefinedRegistered.Other$LowestLifeLostThisTurn",
	} {
		RegisterCountAtom(CountAtom{Name: "count:" + h, GorgeHead: h})
	}
}
