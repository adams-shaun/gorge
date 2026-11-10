package kop

// ExtExileEmbRem (Mysterio, Master of Illusion: the Effect's trigger "exile
// those tokens when it leaves the battlefield"): every object of the firing
// emblem's remembered set (Emblem.CaptureRem) still on the battlefield, in
// the order the emblem remembered them, is exiled. The trigger carries the
// set as the many-object delayed carriers do (trigObj, trigAmt, x, trigMV with
// the versions stamped when it goes on the stack); an object that has left
// since is skipped (CR 400.7). No other field is read. dirac/trig_emblemrem.go.
const ExtExileEmbRem ExtOp = 319

func init() { extOpNames[ExtExileEmbRem] = "ExileEmbRem" }
