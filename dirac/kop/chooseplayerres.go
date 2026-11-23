package kop

// The mid-resolution chosen player (Gluntch, the Bestower: a ChoosePlayer step
// in which its controller chooses any player and remembers the choice, then
// Draw / Token / PutCounter steps defined as the chosen player). gorge effChoosePlayer asks the
// chooser one KChoose over the living players (AliveFrom order, a "player"
// option each) and records the answer on the resolution: Ctx.Chosen holds one
// player, the last chooser's, and RememberChosen$ appends the answer to the
// resolution's remembered list, which `Player.!IsRemembered` excludes from a
// later pool. The kernel keeps both in the resolution (resCtx.cpl): the chosen
// seat and a mask of the remembered players. The as-enters choice
// (ExtChoosePlayer) is the other family and lives in the permanent.

// ExtChoosePlayerRes: the resolving controller chooses one living player and
// the choice is remembered (a ChGenPlayer decision, mode = the seat). C bit 0
// (ChoosePlayerFresh): the pool leaves out the players this resolution already
// chose (Choices$ Player.!IsRemembered). A pool of one is taken unasked (the
// oracle answers a single option in place), an empty pool leaves no chosen
// player. RefChosenPlayerRes reads the answer. dirac/act_chooseplayerres.go.
const ExtChoosePlayerRes ExtOp = 12

// ChoosePlayerFresh is ExtChoosePlayerRes's C bit 0.
const ChoosePlayerFresh = 1

// ChoosePlayerForget is ExtChoosePlayerRes's C bit 1: the answer is the
// resolution's chosen player but is not added to its remembered players (gorge
// appends to the remembered list only under RememberChosen$ True: Valleymaker's
// Choices$-less ChoosePlayer).
const ChoosePlayerForget = 2

func init() { extOpNames[ExtChoosePlayerRes] = "ChoosePlayerRes" }

// RefChosenPlayerRes is a player ref: the player this resolution's last
// ExtChoosePlayerRes chose (gorge Defined$ ChosenPlayer over Ctx.Chosen). None
// before the first choice or after an empty pool.
const RefChosenPlayerRes Ref = 41
