package kop

// ExtRememberAll: every battlefield permanent matching F joins the
// resolution's remembered set, seat 0's first, each seat's in timestamp
// order (PumpAll RememberPumped$ True: gorge effPumpAll remembers each
// permanent its walk pumps). kgen emits it right before the PumpAll it
// belongs to, over the same filter. dirac/act_rememberall.go.
const ExtRememberAll ExtOp = 306

// RememberAllObjOrder (ExtRememberAll C): remember the matches in object
// order instead of the walk's, gorge's g.Objs order for a Defined$ AttachedTo
// selector (Fumble's attachments).
const RememberAllObjOrder = 1

// RememberAllImprint (ExtRememberAll C): do not scan F; Obj names one object
// (RefTriggerObj, the trigger's object) that joins the imprinted set instead,
// gorge Pump ImprintCards$ TriggeredCard (effects/context.go imprint; the
// source keeps it when CardDef.ImpPersist). Oblivious Bookworm.
const RememberAllImprint = 2

// RememberAllImprintCard (ExtRememberAll C, with RememberAllImprint): a
// token is not imprinted (gorge settleChangeZoneMoveAs imprints a non-token
// only). The pre-move half of a Return's Imprint$ (kgen returnImprint).
const RememberAllImprintCard = 4

func init() { extOpNames[ExtRememberAll] = "RememberAll" }
