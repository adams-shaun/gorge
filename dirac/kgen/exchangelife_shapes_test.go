package kgen

import "testing"

// exchangeLifeForms covers api:ExchangeLife. The one-target shape lowers to
// the ExchangeLife extension (Magus of the Mirror, Mirror Universe); the
// two-target arm (Soul Conduit, Axis of Mortality) and the Remember* chains
// (Mister Negative's RememberOwnLoss$; Profane Transfusion and Psychic
// Transfer's RememberDifference$ ride the two-target arm) refuse by name.
var exchangeLifeForms = []formCase{
	{card: "Magus of the Mirror", form: "ValidTgts$ Opponent", lir: "ExchangeLife"},
	{card: "Mirror Universe", form: "ValidTgts$ Opponent", lir: "ExchangeLife"},
	{card: "Soul Conduit", form: "two target players", refused: "ExchangeLife of target slot"},
	{card: "Axis of Mortality", form: "two target players", refused: "ExchangeLife of target slot"},
	{card: "Profane Transfusion", form: "two target players, RememberDifference$", refused: "ExchangeLife of target slot"},
	{card: "Mister Negative", form: "one target, RememberOwnLoss$", refused: "RememberOwnLoss"},
}

func TestExchangeLifeShapes(t *testing.T) { checkForms(t, exchangeLifeForms) }
