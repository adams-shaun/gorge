package oracle

// SCRATCH hunt for the Vivid Crag Flags divergence (ticket projection-vivid-crag-flags).
// Not for commit. Plays the filing loop's old tail-61 card list over a custom
// kgen table (printedDelveSet's way), so the divergence's co-cards are in play.

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

var old61Names = []string{
	"A-Circuit Mender",
	"A-Eyes of the Beholder",
	"A-Jackhammer",
	"A-Omnath, Locus of Creation",
	"Adventure",
	"Advisor",
	"Aenar",
	"Aerie Mystics",
	"Aetherborn",
	"Aethergeode Miner",
	"Aethersquall Ancient",
	"Ajani",
	"Ajani, Valiant Protector",
	"Alabaster Mage",
	"Alien",
	"Ally",
	"Ambush",
	"Anaba Shaman",
	"Andorian",
	"Android",
	"Angel",
	"Antelope",
	"Ape",
	"Arachnus Web",
	"Arcane",
	"Arcane Proxy",
	"Archer",
	"Archon",
	"Argivian Blacksmith",
	"Ark of Blight",
	"Armadillo",
	"Armored",
	"Army",
	"Artificer",
	"Asha's Favor",
	"Assassin",
	"Assembly-Worker",
	"Astartes",
	"Atog",
	"Attended Knight",
	"Attendee",
	"Aura",
	"Aurochs",
	"Avatar",
	"Awakening Zone",
	"Azra",
	"B.O.B.",
	"Back to Nature",
	"Badger",
	"Bahamut",
	"Bajoran",
	"Balloon",
	"Baloth Gorger",
	"Barbarian",
	"Bard",
	"Bartel Runeaxe",
	"Basilisk",
	"Bat",
	"Bear",
	"Beast",
	"Beaver",
	"Beeble",
	"Beetleback Chief",
	"Beholder",
	"Berserker",
	"Betazoid",
	"Bird",
	"Bison",
	"Bladestitched Skaab",
	"Blaster Mage",
	"Blightsoil Druid",
	"Blinkmoth",
	"Boar",
	"Bolian",
	"Book",
	"Boon-Bringer Valkyrie",
	"Borg",
	"Boros Locket",
	"Bottle-Cap Blast",
	"Bottomless Vault",
	"Brainiac",
	"Breach",
	"Briarbridge Tracker",
	"Brikar",
	"Bringer",
	"Brushwagg",
	"C'tan",
	"Cackling Flames",
	"Caitian",
	"Camarid",
	"Camel",
	"Captain America, Skybound",
	"Capybara",
	"Cardassian",
	"Caribou",
	"Carrier",
	"Case",
	"Case of the Shifting Visage",
	"Cat",
	"Catacomb Crocodile",
	"Cathar's Companion",
	"Centaur",
	"Centaur Nurturer",
	"Chef",
	"Child",
	"Child of Gaea",
	"Chimera",
	"Chittering Skitterling",
	"Citizen",
	"Clamfolk",
	"Cleric",
	"Clown",
	"Clue",
	"Cockatrice",
	"Compelling Argument",
	"Confound",
	"Construct",
	"Contaminated Aquifer",
	"Contentious Plan",
	"Coral Trickster",
	"Cow",
	"Coward",
	"Coyote",
	"Crab",
	"Crocodile",
	"Curious Homunculus",
	"Custodes",
	"Cyberman",
	"Cyborg",
	"Cyclops",
	"Dalek",
	"Daredevil, Fearless Fighter",
	"Dauthi",
	"Dawnhart Geist",
	"Deathgrip",
	"Demigod",
	"Demon",
	"Demon of Dark Schemes",
	"Demotion",
	"Denobulan",
	"Deranged Hermit",
	"Deserted Temple",
	"Deserter",
	"Detective",
	"Devil",
	"Devouring Deep",
	"Diminish",
	"Dinosaur",
	"Dirty Wererat",
	"Disappearing Act",
	"Disturbed Burial",
	"Divebomber Griffin",
	"Djinn",
	"Doctor",
	"Doctor Strange, Master of Magic",
	"Dog",
	"Dragon",
	"Dragon Tyrant",
	"Drake",
	"Dreadmalkin",
	"Dreadnought",
	"Drix",
	"Dromoka Monument",
	"Drone",
	"Dross Harvester",
	"Druid",
	"Dryad",
	"Dryad Greenseeker",
	"Duck",
	"Dueling Coach",
	"Durkwood Baloth",
	"Dwarf",
	"Dwarven Bloodboiler",
	"Earthshaker",
	"Echidna",
	"Echoing Return",
	"Edosian",
	"Efreet",
	"Egg",
	"El-Aurian",
	"Elder",
	"Eldrazi",
	"Elemental",
	"Elemental Summoning",
	"Elephant",
	"Elf",
	"Elk",
	"Ellyn Harbreeze, Busybody",
	"Elvish Handservant",
	"Elvish Warrior",
	"Ember Beast",
	"Employee",
	"Equipment",
	"Eternal",
	"Ethereal Guidance",
	"Excalbian",
	"Exemplar of Strength",
	"Exquisite Blood",
	"Exuberant Wolfbear",
	"Eye",
	"Faerie",
	"Fallaji Vanguard",
	"Fangren Hunter",
	"Felidar Umbra",
	"Fell Flagship",
	"Ferengi",
	"Ferret",
	"Fighter",
	"Filigree Crawler",
	"Final Revels",
	"Fish",
	"Five Hundred Year Diary",
	"Flagbearer",
	"Flickering Spirit",
	"Fog of Gnats",
	"Font of Progress",
	"Force of Vigor",
	"Forest",
	"Fortress Cyclops",
	"Fox",
	"Fractal",
	"Frenetic Raptor",
	"Freyalise",
	"Freyalise, Llanowar's Fury",
	"Frog",
	"Fungus",
	"Gamer",
	"Gamma",
	"Gargoyle",
	"Genju of the Spires",
	"Germ",
	"Ghor-Clan Bloodscale",
	"Giant",
	"Giraffe",
	"Gird for Battle",
	"Gith",
	"Glass",
	"Glimmer",
	"Glorifier of Dusk",
	"Glory of Warfare",
	"Gnoll",
	"Gnome",
	"Goat",
	"Gobbling Ooze",
	"Goblin",
	"Goblin Tunneler",
	"God",
	"Gods' Eye, Gate to the Reikai",
	"Golden Wish",
	"Goldmeadow Dodger",
	"Golem",
	"Gorgon",
	"Gorilla",
	"Gorn",
	"Graveborn",
	"Gravel-Hide Goblin",
	"Gremlin",
	"Griffin",
	"Grimdancer",
	"Guardian Shield-Bearer",
	"Guest",
	"Hag",
	"Halfling",
	"Hamster",
	"Harpy",
	"Healing Salve",
	"Hedgehog",
	"Hedron Detonator",
	"Hellcarver Demon",
	"Hellion",
	"Henge of Ramos",
	"Hero",
	"Highland Game",
	"Hippo",
	"Hippogriff",
	"Hirogen",
	"Homarid",
	"Homunculus",
	"Hopeful Eidolon",
	"Horror",
	"Horse",
	"Horsehead",
	"Howlpack Wolf",
	"Human",
	"Hunted Ghoul",
	"Hydra",
	"Hyena",
	"Ihsan's Shade",
	"Ill-Tempered Loner",
	"Illumination",
	"Illusion",
	"Illusionary Wall",
	"Illyrian",
	"Imp",
	"Incarnation",
	"Inhuman",
	"Inkling",
	"Inquisitor",
	"Insect",
	"Irregular Cohort",
	"Island",
	"Jackal",
	"Jagwasp Swarm",
	"Jellyfish",
	"Jem'Hadar",
	"Jenara, Asura of War",
	"Jon Irenicus, the Exile",
	"Judge",
	"Juggernaut",
	"Jund Battlemage",
	"Kangaroo",
	"Kavu",
	"Kelpien",
	"Killbot",
	"Kiri-Onna",
	"Kirin",
	"Kithkin",
	"Kjeldoran Pride",
	"Klingon",
	"Klothys's Design",
	"Knight",
	"Kobold",
	"Kor",
	"Kraken",
	"Kranioceros",
	"Kree",
	"Kura, the Boundless Sky",
	"Kwejian",
	"Lamia",
	"Lammasu",
	"Lanthanite",
	"Lattice Library",
	"Leech",
	"Leech Gauntlet",
	"Lemur",
	"Lesson",
	"Leviathan",
	"Lhurgoyf",
	"Licid",
	"Liliana's Scorn",
	"Lizard",
	"Llama",
	"Lobster",
	"Lord",
	"Loyal Retainers",
	"Lucky Clover",
	"Maga, Traitor to Mortals",
	"Mahadi, Emporium Master",
	"Mammoth",
	"Manticore",
	"Master Thief",
	"Masticore",
	"Matsu-Tribe Birdstalker",
	"Maximize Velocity",
	"Medusan",
	"Meishin, the Mind Cage",
	"Melkotian",
	"Mercenary",
	"Mercenary Informer",
	"Merfolk",
	"Merfolk Observer",
	"Merfolk of the Depths",
	"Metathran",
	"Mindless-One",
	"Minion",
	"Minotaur",
	"Mire Shade",
	"Mirkwood Channeler",
	"Mirran Crusader",
	"Mishra's Domination",
	"Mist Raven",
	"Mite",
	"Molder Slug",
	"Mole",
	"Monger",
	"Mongoose",
	"Monk",
	"Monkey",
	"Monstrify",
	"Moogle",
	"Moonfolk",
	"Mount",
	"Mountain",
	"Mouse",
	"Mutant",
	"Mwonvuli Acid-Moss",
	"Myr",
	"Mystic",
	"Mystic Skull",
	"N'Yami-Class Mother Ship",
	"Na'kuhl",
	"Najal, the Storm Runner",
	"Narwhal",
	"Nautilus",
	"Necron",
	"Nephilim",
	"Nightmare",
	"Nightstalker",
	"Ninja",
	"Noble",
	"Noggle",
	"Nomad",
	"Nymph",
	"Obsessive Skinner",
	"Ocampa",
	"Octopus",
	"Officer",
	"Ogre",
	"One with Nature",
	"Ooze",
	"Orb",
	"Orc",
	"Orcish Vandal",
	"Orgg",
	"Orion",
	"Otter",
	"Ouphe",
	"Outbreak",
	"Overwhelming Denial",
	"Oyster",
	"Pact of the Titan",
	"Pangolin",
	"Panic Attack",
	"Patriarch's Desire",
	"Peasant",
	"Pegasus",
	"Pegasus Courser",
	"Pentavite",
	"Performer",
	"Pest",
	"Phantom Warrior",
	"Phelddagrif",
	"Phoenix",
	"Phyrexian",
	"Pilgrim of the Fires",
	"Pillar Tombs of Aku",
	"Pilot",
	"Pincher",
	"Pirate",
	"Pirate's Pillage",
	"Plague Spores",
	"Plains",
	"Plant",
	"Plated Spider",
	"Platypus",
	"Political Trickery",
	"Porcupine",
	"Possum",
	"Pouncing Lynx",
	"Praetor",
	"Predict",
	"Press for Answers",
	"Primarch",
	"Prism",
	"Processor",
	"Protector of Gondor",
	"Putrid Leech",
	"Rabbit",
	"Raccoon",
	"Ranger",
	"Rat",
	"Ravenous Lindwurm",
	"Razor Swine",
	"Rebel",
	"Reflection",
	"Relic Robber",
	"Reprobation",
	"Repudiate",
	"Resistance Reunited",
	"Returned Pastcaller",
	"Reverberate",
	"Rhino",
	"Rigger",
	"Rith, the Awakener Avatar",
	"Robot",
	"Rogue",
	"Romulan",
	"Rowan Treefolk",
	"Sable",
	"Saga",
	"Sage's Row Denizen",
	"Saiba Trespassers",
	"Salamander",
	"Samurai",
	"Sanctuary Smasher",
	"Sand",
	"Saproling",
	"Satyr",
	"Scarecrow",
	"Scientist",
	"Scion",
	"Scorpion",
	"Scout",
	"Scrounger of Souls",
	"Sculpture",
	"Seal",
	"Secrets of the Golden City",
	"Segmented Krotiq",
	"Seismic Tutelage",
	"Serf",
	"Serpent",
	"Servo",
	"Shade",
	"Shaman",
	"Shaper Apprentice",
	"Shapeshifter",
	"Shard of the Nightbringer",
	"Shark",
	"Shattered Crypt",
	"Shatterskull Smashing",
	"Sheep",
	"Shi'ar",
	"Sigiled Paladin",
	"Simic Locket",
	"Siren",
	"Skeleton",
	"Skinwing",
	"Skrull",
	"Skunk",
	"Sky-Blessed Samurai",
	"Skyshroud Claim",
	"Slith",
	"Sliver",
	"Sloth",
	"Slug",
	"Smelted Chargebug",
	"Smogsteed Rider",
	"Snail",
	"Snake",
	"Soldier",
	"Soltari",
	"Somber Hoverguard",
	"Sorcerer",
	"Spawn",
	"Specter",
	"Spellshaper",
	"Sphinx",
	"Spider",
	"Spike",
	"Spirit",
	"Splinter",
	"Splinter Twin",
	"Sponge",
	"Spotter Thopter",
	"Spy",
	"Squid",
	"Squirrel",
	"Stained",
	"Starfish",
	"Stronghold Biologist",
	"Stupor",
	"Summon the School",
	"Summon: Good King Mog XII",
	"Surrakar",
	"Survivor",
	"Swamp",
	"Symbiote",
	"Synth",
	"Syphon Life",
	"Talaxian",
	"Talosian",
	"Tamarian",
	"Tameshi, Reality Architect",
	"Tarfire",
	"Tavern Swindler",
	"Teddy",
	"Tellarite",
	"Tempered in Solitude",
	"Temporal Eddy",
	"Tentacle",
	"Territorial Maro",
	"Tetravite",
	"Thalakos",
	"Thalakos Sentry",
	"Thallid Omnivore",
	"Thanos, Death's Consort",
	"The Wasp, Winsome Avenger",
	"Tholian",
	"Thopter",
	"Thrash of Raptors",
	"Thrull",
	"Tiefling",
	"Timbermaw Larva",
	"Time",
	"Time Reversal",
	"Tosk",
	"Towering Viewpoint",
	"Toy",
	"Treasure",
	"Treefolk",
	"Tribble",
	"Trigon of Corruption",
	"Trill",
	"Trilobite",
	"Triskelavite",
	"Troll",
	"Turtle",
	"Tymaret, Chosen from Death",
	"Tyranid",
	"Unicorn",
	"Unity of Purpose",
	"Utrom",
	"Valentina Allegra de Fontaine",
	"Vampire",
	"Vampire Revenant",
	"Vanguard",
	"Vanguard of Brimaz",
	"Varmint",
	"Vau-N'Akat",
	"Vedalken",
	"Vehicle",
	"Vexing Bauble",
	"Viashino Runner",
	"Villain",
	"Viridian Corrupter",
	"Vivid Crag",
	"Vivisection Evangelist",
	"Voice of Duty",
	"Voice of the Woods",
	"Volver",
	"Vorta",
	"Vrock",
	"Vulcan",
	"Wall",
	"Walrus",
	"Wanderer's Strike",
	"Warlock",
	"Warrant",
	"Warrior",
	"Weasel",
	"Weatherseed Totem",
	"Weird",
	"Werebear",
	"Werewolf",
	"Whale",
	"Whiptail Wurm",
	"Whirlwind Technique",
	"Windbrisk Raptor",
	"Without Weakness",
	"Wizard",
	"Wizened Cenn",
	"Wolf",
	"Wolverine",
	"Wombat",
	"Woodland Wanderer",
	"World",
	"Worm",
	"Wraith",
	"Wurm",
	"Xindi",
	"Yavimaya's Embrace",
	"Yeti",
	"Zombie",
	"Zubera",
}

func TestScratchOld61Hunt(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var cand []string
	cand = append(cand, "Vivid Crag")
	for _, name := range old61Names {
		c, ok := reg.Lookup(name)
		if !ok {
			continue
		}
		var oracle string
		for _, f := range c.Faces {
			oracle += f.Oracle + " "
		}
		lower := strings.ToLower(oracle)
		if strings.Contains(lower, "mana") || strings.Contains(oracle, "{T}") ||
			strings.Contains(lower, "untap") || strings.Contains(lower, "charge") ||
			strings.Contains(lower, "counter") {
			cand = append(cand, name)
		}
	}
	if len(cand) > 90 {
		cand = cand[:90]
	}
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	list := strings.Join(cand, "\n") + "\nPlains\nIsland\nSwamp\nMountain\nForest\n"
	tmp := t.TempDir()
	listFile := tmp + "/cards.txt"
	if err := os.WriteFile(listFile, []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	out := tmp + "/out"
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", listFile, "-pool", "old61hunt", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(out + "/old61hunt.ktab")
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	var inSet []string
	for i := 0; i < set.NumCards(); i++ {
		inSet = append(inSet, set.Names()[i])
	}
	if !containsStr(inSet, "Vivid Crag") {
		t.Fatalf("the custom table holds no Vivid Crag: %d cards", len(inSet))
	}
	t.Logf("custom table: %d cards, %d candidates", len(inSet), len(cand))
	rng := rand.New(rand.NewPCG(77, 5))
	basics := [...]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	played, refused := 0, 0
	divs := map[string]int{}
	for pair := 0; pair < 260; pair++ {
		var kd [2][]cardset.Entry
		for s := 0; s < 2; s++ {
			kd[s] = append(kd[s], cardset.Entry{Name: "Vivid Crag", Count: 4})
			for i := 0; i < 16; i++ {
				kd[s] = append(kd[s], cardset.Entry{Name: basics[rng.IntN(5)], Count: 1})
			}
			for i := 0; i < 20; i++ {
				kd[s] = append(kd[s], cardset.Entry{Name: inSet[rng.IntN(len(inSet))], Count: 1})
			}
		}
		tbl, _, _, err := set.Matchup(kd[0], kd[1])
		if err != nil {
			refused++
			continue
		}
		var decks [2][]*cards.Card
		ok := true
		for s := 0; s < 2 && ok; s++ {
			for _, e := range kd[s] {
				c, found := reg.Lookup(e.Name)
				if !found {
					ok = false
					break
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				ok = false
			}
		}
		if !ok {
			continue
		}
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"h-a", "h-b"}, decks, 0)
		if err != nil {
			continue
		}
		pol := func(seed uint64) Policy {
			if pair%2 == 0 {
				return ExplorePolicy(seed)
			}
			return pkPolicy(seed)
		}
		for seed := uint64(1); seed <= 2; seed++ {
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				break
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				break
			}
			res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 45})
			CloseSide(k)
			if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
				divs["stall:"+res.Stall]++
				break
			}
			if d := res.Div; d != nil {
				divs[d.What+" "+d.Field]++
				t.Logf("pair %d seed %d diverged at %d: %s %s %v %s", pair, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				crag := false
				poolDiff := false
				for _, tok := range strings.Split(d.Field+" "+d.Detail+" "+d.What, " ") {
					if i := strings.Index(tok, "Cards["); i >= 0 {
						rest := tok[i+6:]
						if j := strings.Index(rest, "]"); j > 0 {
							if idx, okn := atoiOK(rest[:j]); okn {
								if o := g.E.G.Obj(state.ObjID(idx)); o != nil && o.Face().Name == "Vivid Crag" && strings.Contains(tok, ".Flags") {
									crag = true
								}
							}
						}
					}
					if strings.Contains(tok, "Pool[") {
						poolDiff = true
					}
				}
				if crag || poolDiff {
					if strings.Contains(d.Field, ".Flags") {
						rp := NewRepro(m, seed, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 45}, &res)
						rp.Pool = "custom"
						rp.Why = "gorge taps Vivid Crag for mana where the kernel taps another source"
						if err := rp.Write("/home/sadams/projects/gorge/.worktrees/agent-20261116T003840Z-b4605993/.ds4/scratch/vivid_crag_flags.repro.json"); err != nil {
							t.Fatal(err)
						}
						t.Log("wrote .ds4/scratch/vivid_crag_flags.repro.json")
					}
					for idx := 40; idx < 60; idx++ {
						if o := g.E.G.Obj(state.ObjID(idx)); o != nil {
							t.Logf("pair %d seed %d: Cards[%d] is %q zone=%d tapped=%v", pair, seed, idx, o.Face().Name, o.Zone, o.Tapped)
						} else {
							t.Logf("pair %d seed %d: Cards[%d] is nil", pair, seed, idx)
						}
					}
					lo := max(d.At-20, 0)
					t.Logf("pair %d seed %d last actions %v", pair, seed, actionStrings(res.Actions[lo:d.At]))
					t.Logf("gorge pool %v", g.E.G.Players[0].Pool)
					for s2 := 0; s2 < 2; s2++ {
						var dl []string
						for _, e := range kd[s2] {
							dl = append(dl, e.Name)
						}
						t.Logf("pair %d deck %d: %s", pair, s2, strings.Join(dl, ","))
					}
					t.Fatalf("VIVID CRAG DIVERGENCE: %s %s %v %s\nref %v\nker %v", d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				break
			}
		}
		played++
	}
	t.Logf("hunt done: %d played, %d refused", played, refused)
	for k, v := range divs {
		t.Logf("class %q: %d", k, v)
	}
}

func containsStr(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

// replayAndShrink drives the recorded repro on its own custom table and shrinks
// the action prefix while the divergence class holds.
func TestScratchReproReplayShrink(t *testing.T) {
	t.Parallel()
	dirac.DebugPlanTrace = true
	r, err := ReadRepro("/home/sadams/projects/gorge/.worktrees/agent-20261116T003840Z-b4605993/.ds4/scratch/vivid_crag_flags.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	dirac.DebugPlanTag = "run1"
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	res0 := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res0.Div == nil {
		t.Fatalf("the recording replayed clean on its own table: %d actions", len(r.Actions))
	}
	t.Logf("replay diverged at %d: %s %s %v %s", res0.Div.At, res0.Div.What, res0.Div.Field, res0.Div.Diff, res0.Div.Detail)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	rc.MaxDecisions = res0.Div.At // stop at the divergence: the states below are its states
	dirac.DebugPlanTag = "run2"
	k2, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	_ = LockstepLed(m, r.Seed, g, k2, &ScriptPolicy{Actions: r.Actions[:res0.Div.At-1]}, rc)
	defer CloseSide(k2)
	var gp, kp2 Projection
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("kernel Project panicked: %v", r)
			}
		}()
		g.Project(&gp)
		k2.Project(&kp2)
		for _, i := range []int{43, 51, 44, 52} {
			gr, kr := gp.Cards[i], kp2.Cards[i]
			t.Logf("proj %d: gorge zone=%d flags=%d pos=%d | kernel zone=%d flags=%d pos=%d", i, gr.Zone, gr.Flags, gr.Pos, kr.Zone, kr.Flags, kr.Pos)
		}
		t.Logf("proj pools gorge=%v kernel=%v", gp.Pool, kp2.Pool)
		for i := 40; i <= 60; i++ {
			oid := g.id.obj[i]
			o := g.E.G.Obj(oid)
			if o == nil {
				continue
			}
			gr, kr := gp.Cards[i], kp2.Cards[i]
			t.Logf("map inst=%d gorgeobj=%d %q gorgezone=%d gorgetap=%v | proj g z=%d f=%d p=%d k z=%d f=%d p=%d",
				i, oid, o.Face().Name, o.Zone, o.Tapped, gr.Zone, gr.Flags, gr.Pos, kr.Zone, kr.Flags, kr.Pos)
		}
	}()
	// the state just before the diverging cast
	t.Logf("PRE pool p0 %v p1 %v", g.E.G.Players[0].Pool, g.E.G.Players[1].Pool)
	for seat := 0; seat < 2; seat++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(seat)) {
			o := g.E.G.Obj(id)
			if o == nil {
				continue
			}
			t.Logf("PRE bf%d %d %q tapped=%v", seat, id, o.Face().Name, o.Tapped)
		}
		for _, id := range g.E.G.Zone(state.ZHand, state.PlayerID(seat)) {
			o := g.E.G.Obj(id)
			if o != nil {
				t.Logf("PRE hand%d %d %q", seat, id, o.Face().Name)
			}
		}
	}
	if o := g.E.G.Obj(state.ObjID(64)); o != nil {
		t.Logf("cast source #64 is %q", o.Face().Name)
	}
	CloseSide(k)

	// gorge-only replay of the diverging cast: dump p1's tapped bits before
	// and after applying the last recorded action (the Dryad Greenseeker
	// cast), so the payment's taps are measured, not inferred.
	g3, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	tapDump := func(tag string) {
		for seat := 0; seat < 2; seat++ {
			for _, id := range g3.E.G.Zone(state.ZBattlefield, state.PlayerID(seat)) {
				o := g3.E.G.Obj(id)
				if o == nil {
					continue
				}
				if o.Tapped {
					t.Logf("%s gorge bf%d %d %q TAPPED", tag, seat, id, o.Face().Name)
				}
			}
			t.Logf("%s gorge pool p%d %v", tag, seat, g3.E.G.Players[seat].Pool)
			for _, id := range g3.E.G.Zone(state.ZHand, state.PlayerID(seat)) {
				o := g3.E.G.Obj(id)
				if o != nil {
					t.Logf("%s gorge hand%d %d %q", tag, seat, id, o.Face().Name)
				}
			}
		}
		t.Logf("%s gorge stack %d", tag, len(g3.E.G.Stack))
	}
	pol3 := &ScriptPolicy{Actions: r.Actions[:res0.Div.At-1]}
	for i := 0; i < res0.Div.At-1; i++ {
		d := g3.Step()
		a := pol3.Pick(&PickCtx{D: &d, Seq: i})
		g3.Apply(a)
	}
	t.Logf("script skipped=%d next=%d err=%v", pol3.Skipped, pol3.next, g3.Err())
	tapDump("pre")
	d588 := g3.Step()
	t.Logf("decision 588: kind=%v player=%v options=%v", d588.Kind, d588.Player, actionStrings(d588.Options))
	last := r.Actions[res0.Div.At-1]
	t.Logf("last action %v", last)
	g3.Apply(last)
	t.Logf("after apply err=%v", g3.Err())
	for si, sid := range g3.E.G.Stack {
		o := g3.E.G.Obj(sid)
		if o != nil {
			t.Logf("post stack[%d] obj=%d %q zone=%d", si, sid, o.Face().Name, o.Zone)
		}
	}
	if o := g3.E.G.Obj(state.ObjID(64)); o != nil {
		t.Logf("post obj64 zone=%d", o.Zone)
	}
	tapDump("post")
	CloseSide(g3)
	return
}
