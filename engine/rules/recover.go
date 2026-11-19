package rules

import "github.com/adams-shaun/gorge/engine/effects"

// Recover (kw:Recover, CR 702.59: "Recover [cost] (When a creature is put
// into your graveyard from the battlefield, you may pay [cost]. If you do,
// return this card from your graveyard to your hand. Otherwise, exile this
// card.)") is expanded by cards into an ordinary ChangesZone trigger living
// in the graveyard (engine/cards/kw_recover.go), so it is ordered and placed
// with every other trigger and can be responded to before it resolves. This
// file owns the resolution-time election, the Echo flow's shape
// (engine/rules/echo.go): pay through the shared pool-only payment window
// (resPayFund, paymentManaAsk) or decline. The decline arm and any failed
// payment exile the card — CR 702.59's "otherwise, exile this card" — and
// the pay arm returns it from the graveyard to the hand. A cost this build
// cannot price is skipped LOUDLY (one Note, the card stays in the
// graveyard), the Echo convention: a silent exile every death would be
// flatly wrong.
//

func init() {
	effects.RegisterNonAPI("kw:Recover")
}
