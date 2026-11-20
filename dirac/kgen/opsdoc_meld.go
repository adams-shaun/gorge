package kgen

// opsMeld documents OpExt ExtMeld (dirac/face_meld.go).
var opsMeld = []opsDoc{
	{"ext:Meld", "OpExt ExtMeld (Obj the source, F the partner's filter)", "api Meld (CR 701.42a; gorge effects/meld_api.go effMeld, effects.Meld): the source, a Meld result card (CardDef.FaceMode FaceMeld) on the battlefield under the resolving controller, finds the first battlefield permanent of F (the pair's other card by name, YouCtrl, seat then battlefield order); both are exiled as one zone batch, source first; unless the pair is a pair and neither is a token or copy (then both stay in exile, CR 701.42c) the partner parks in ZoneNone, the result turns to its melded face (Faces[1]) and enters under the controller (a normal entry). The pair is one record of the cold `meld` region; when the result leaves the battlefield the partner goes to the same zone in its owner's zone (gorge events.unmeld), silently. The result's mana cost is the sum of the two front faces' mana values (CR 712.8g); Tapped$/Attacking$ and a Commander pool refuse (part 2). Graf Rats, Hanweir Battlements, Vanille, Cheerful l'Cie."},
}
