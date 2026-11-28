package ir

import "slices"

// The closed word lists a Filter's Word is checked against (ValidateFilter).
// Each list is sorted in byte order, so a lookup is a binary search; the
// words are game vocabulary, spelled as the rules spell them.

// WordKind says which type list a word belongs to.
type WordKind uint8

const (
	WordNone WordKind = iota
	WordCardType
	WordSupertype
	WordSubtype
	wordKindCount
)

var wordKindNames = [...]string{
	WordNone:      "none",
	WordCardType:  "card-type",
	WordSupertype: "supertype",
	WordSubtype:   "subtype",
}

// String returns the lower-kebab name of k.
func (k WordKind) String() string { return enumString(wordKindNames[:], k, "WordKind") }

// TypeWord says which type list holds w: card type, supertype or subtype.
// The match is exact and case-sensitive; any other word is WordNone.
func TypeWord(w string) WordKind {
	switch {
	case inList(cardTypeWords, w):
		return WordCardType
	case inList(supertypeWords, w):
		return WordSupertype
	case inList(subtypeWords, w):
		return WordSubtype
	}
	return WordNone
}

// IsKeyword reports whether w is a keyword's rules name, with only its first
// word capitalised ("First strike").
func IsKeyword(w string) bool { return inList(keywordWords, w) }

// IsCounterKind reports whether w is a counter kind's printed name in lower
// case ("+1/+1", "loyalty").
func IsCounterKind(w string) bool { return inList(counterKindWords, w) }

func inList(l []string, w string) bool {
	_, ok := slices.BinarySearch(l, w)
	return ok
}

// cardTypeWords are the card types (CR 205.2a).
var cardTypeWords = []string{
	"Artifact", "Battle", "Conspiracy", "Creature", "Dungeon", "Enchantment", "Instant", "Kindred",
	"Land", "Phenomenon", "Plane", "Planeswalker", "Scheme", "Sorcery", "Vanguard",
}

// supertypeWords are the supertypes (CR 205.4a).
var supertypeWords = []string{
	"Basic", "Legendary", "Ongoing", "Snow", "World",
}

// subtypeWords are the subtypes: every type word an object in this engine's
// corpus can carry (engine/effects predicateTypeWords) that is neither a card
// type nor a supertype.
var subtypeWords = []string{
	"Abyss", "Adventure", "Advisor", "Aenar", "Aetherborn", "Ajani", "Alara", "Alfava", "Alien",
	"Ally", "Aminatou", "Amonkhet", "Amsterdam", "Andorian", "Android", "Androzani", "Angel", "Angrath",
	"Antausia", "Antelope", "Apalapucia", "Ape", "Arcane", "Arcavios", "Archer", "Archon", "Arkhos",
	"Arlinn", "Armadillo", "Armored", "Army", "Artificer", "Arzakon", "Ashiok", "Assassin",
	"Assembly-Worker", "Astartes", "Asylum", "Atog", "Attraction", "Aura", "Aurochs", "Avatar",
	"Avishkar", "Azgol", "Azra", "B.O.B.", "Background", "Badger", "Bahamut", "Bajoran", "Balloon",
	"Barbarian", "Bard", "Basilisk", "Basri", "Bat", "Bear", "Beast", "Beaver", "Beeble", "Beholder",
	"Belenon", "Berserker", "Beserker", "Betazoid", "Bird", "Bison", "Blinkmoth", "Blood", "Boar",
	"Bobblehead", "Bolas", "Bolas's", "Bolian", "Book", "Borg", "Brainiac", "Brikar", "Bringer",
	"Brushwagg", "C'tan", "Caitian", "Calix", "Camarid", "Camel", "Capenna", "Capybara", "Cardassian",
	"Caribou", "Carrier", "Cartouche", "Case", "Cat", "Cave", "Centaur", "Chandra", "Chef", "Chicago", "Child", "Chimera",
	"Chorus", "Citizen", "Clamfolk", "Class", "Cleric", "Clown", "Clue", "Cockatrice", "Comet",
	"Communicator", "Construct", "Contraption", "Cow", "Coward", "Coyote", "Crab", "Cridhe",
	"Crocodile", "Curse", "Custodes", "Cyberman", "Cyborg", "Cyclops", "Dack", "Dakkon", "Dalek",
	"Daretti", "Darillium", "Dauthi", "Davriel", "Deb", "Dellian", "Demigod", "Demon", "Denobulan",
	"Desert", "Deserter", "Detective", "Devil", "Dihada", "Dinosaur", "Djinn", "Doctor", "Dog",
	"Dominaria", "Domri", "Dovin", "Dragon", "Drake", "Dreadnought", "Drix", "Drone", "Druid",
	"Dryad", "Duck", "Duskmourn", "Dwarf", "Dyfed", "Earth", "Echidna", "Echoir", "Edosian", "Efreet", "Egg",
	"El-Aurian", "Elder", "Eldraine", "Eldrazi", "Elemental", "Elephant", "Elf", "Elk", "Ellywick",
	"Elminster", "Elspeth", "Employee", "Equilor", "Equipment", "Ergamon", "Ersta", "Estrid",
	"Eternal", "Excalbian", "Eye", "Fabacin", "Faerie", "Ferengi", "Feroz", "Ferret", "Fighter",
	"Fiora", "Fish", "Flagbearer", "Food", "Forest", "Fortification", "Fox", "Fractal", "Freyalise",
	"Frog", "Fungus", "Gallifrey", "Gamer", "Gamma", "Gargantikar", "Gargoyle", "Garruk", "Gate",
	"Germ", "Giant", "Gideon", "Giraffe", "Gith", "Glass", "Glimmer", "Gnoll", "Gnome", "Goat",
	"Gobakhan", "Goblin", "God", "Gold", "Golem", "Gorgon", "Gorilla", "Gorn", "Graveborn",
	"Greensleeves", "Gremlin", "Griffin", "Grist", "Guest", "Guff", "Hag", "Halfling", "Hamster",
	"Harpy", "Heartwood", "Hedgehog", "Hell", "Hellion", "Hero", "Hippo", "Hippogriff", "Hirogen", "Homarid",
	"Homunculus", "Horror", "Horse", "Horsehead", "Host", "Huatli", "Human", "Hydra", "Hyena",
	"Ikoria", "Illusion", "Illyrian", "Imp", "Incarnation", "Incubator", "Infinity", "Inhuman",
	"Inkling", "Innistrad", "Inquisitor", "Insect", "Inzerva", "Iquatana", "Ir", "Island", "Ixalan",
	"Jace", "Jackal", "Jared", "Jaya", "Jellyfish", "Jem'Hadar", "Jeska", "Judge", "Juggernaut", "Junction", "Junk",
	"Kaito", "Kaldheim", "Kamigawa", "Kandoka", "Kangaroo", "Karn", "Karsus", "Kasmina", "Kavu",
	"Kaya", "Kelpien", "Kephalai", "Key", "Killbot", "Kinshala", "Kiora", "Kirin", "Kithkin",
	"Klingon", "Knight", "Kobold", "Kolbahan", "Kor", "Koth", "Kraken", "Kree", "Kwejian", "Kylem",
	"Kyneth", "Lair", "Lamia", "Lammasu", "Lander", "Lanthanite", "Las", "Leech", "Lemur", "Lesson",
	"Leviathan", "Lhurgoyf", "Library", "Licid", "Liliana", "Lizard", "Llama", "Lobster", "Locus",
	"Lolth", "Lord", "Lorwyn", "Lukka", "Luvion", "MagicCon", "Mammoth", "Manticore", "Map", "Mars", "Master",
	"Masticore", "Meditation", "Medusan", "Melkotian", "Mercadia", "Mercenary", "Merfolk",
	"Metathran", "Metraxis", "Mindless-One", "Mine", "Minion", "Minor", "Minotaur", "Minsc",
	"Mirrodin", "Mite", "Moag", "Mole", "Monger", "Mongoose", "Mongseng", "Monk", "Monkey",
	"Monopoly", "Moogle", "Moon", "Moonfolk", "Mordenkainen", "Mount", "Mountain", "Mouse",
	"Muraganda", "Mutagen", "Mutant", "Mutter's", "Myr", "Mystic", "Na'kuhl", "Nahiri", "Narset",
	"Nautilus", "Nebula", "Necron", "Necros", "Nephilim", "New", "Nightmare", "Nightstalker", "Niko",
	"Ninja", "Nissa", "Nixilis", "Noble", "Noggle", "Nomad", "Nymph", "Ocampa", "Octopus", "Officer",
	"Ogre", "Oko", "Omen", "Omenpath", "Ooze", "Orb", "Orc", "Orgg", "Orion", "Otter", "Ouphe",
	"Outside", "Ox", "Oyster", "Pangolin", "Peasant", "Pegasus", "Pentavite", "Performer", "Pest",
	"Phelddagrif", "Phoenix", "Phyrexia", "Phyrexian", "Pilot", "Pincher", "Pirate", "Plains", "Plan",
	"Planet", "Plant", "Platypus", "Porcupine", "Possum", "Power-Plant", "Powerstone", "Praetor",
	"Primarch", "Prism", "Processor", "Pyrulea", "Q", "Qu", "Quintorius", "Rabbit", "Rabiah",
	"Raccoon", "Ral", "Ranger", "Rat", "Rath", "Ravnica", "Realm", "Rebel", "Reflection", "Regatha",
	"Rhino", "Rigger", "Robot", "Rogue", "Role", "Romulan", "Room", "Rowan", "Rune", "Sable", "Saga",
	"Saheeli", "Salamander", "Samurai", "Samut", "Sand", "Saproling", "Sarkhan", "Satyr", "Scarecrow",
	"Scientist", "Scion", "Scorpion", "Scout", "Sculpture", "Seal", "Segovia", "Serf", "Serpent",
	"Serra", "Serra's", "Servo", "Shade", "Shadowmoor", "Shaman", "Shandalar", "Shapeshifter",
	"Shard", "Shark", "Sheep", "Shenmeng", "Shi'ar", "Shrine", "Siege", "Sifa", "Siren", "Sivitri",
	"Skaro", "Skeleton", "Skrull", "Skunk", "Slith", "Sliver", "Sloth", "Slug", "Snail", "Snake",
	"Soldier", "Soltari", "Sorcerer", "Sorin", "Spacecraft", "Spawn", "Specter", "Spellshaper",
	"Sphere", "Sphinx", "Spider", "Spike", "Spiral", "Spirit", "Splinter", "Sponge", "Spy", "Squid",
	"Squirrel", "Stained", "Starfish", "Stickers", "Stone", "Surrakar", "Survivor", "Svega", "Swamp", "Symbiote", "Synth",
	"Szat", "Talaxian", "Talosian", "Tamarian", "Tamiyo", "Tarkir", "Tasha", "Teddy", "Teferi",
	"Tellarite", "Tentacle", "Tetravite", "Teyo", "Tezzeret", "Thalakos", "The", "Theros", "Tholian",
	"Thomil", "Thopter", "Thrull", "Thunder", "Tibalt", "Tiefling", "Time", "Time Lord", "Tosk", "Tower", "Town", "Toy",
	"Trap", "Treasure", "Treefolk", "Trenzalore", "Tribble", "Trill", "Trilobite", "Triskelavite",
	"Troll", "Turtle", "Tyranid", "Tyvar", "Ugin", "Ulgrotha", "Undercity", "Unicorn", "Unknown",
	"Urza", "Urza's", "Utrom", "Valla", "Vampire", "Varmint", "Vau-N'Akat", "Vedalken", "Vegas", "Vehicle", "Vengeance",
	"Venser", "Vibranium", "Villain", "Vivien", "Volver", "Vorta", "Vraska", "Vronos", "Vryn",
	"Vulcan", "Wall", "Walrus", "Warlock", "Warrior", "Weasel", "Weird", "Werewolf", "Whale",
	"Wildfire", "Will", "Windgrace", "Wizard", "Wolf", "Wolverine", "Wombat", "Worm", "Worzel",
	"Wraith", "Wrenn", "Wurm", "Xenagos", "Xerex", "Xindi", "Yanggu", "Yanling", "Yeti", "You",
	"Zariel", "Zendikar", "Zhalfir", "Zombie", "Zubera",
}

// keywordWords are the keyword names a filter tests for.
var keywordWords = []string{
	"Augment", "Awaken", "Cascade", "Convoke", "Cumulative upkeep", "Cycling", "Deathtouch",
	"Decayed", "Defender", "Disturb", "Double strike", "Embalm", "Eternalize", "Fading",
	"First strike", "Flanking", "Flash", "Flashback", "Flying", "Forestwalk", "Foretell",
	"Freerunning", "Haste", "Hexproof", "Horsemanship", "Indestructible", "Infect", "Islandwalk",
	"Kicker", "Level up", "Lifelink", "Madness", "Megamorph", "Menace", "Modular", "Morph",
	"Mountainwalk", "Multikicker", "Mutate", "Partner", "Persist", "Plainswalk", "Protection",
	"Rampage", "Reach", "Shadow", "Skulk", "Soulbond", "Suspend", "Swampwalk", "Toxic", "Trample",
	"Unearth", "Vigilance", "Ward", "Warp",
}

// counterKindWords are the counter kinds a filter tests for.
var counterKindWords = []string{
	"+1/+1", "-1/-1", "aim", "bounty", "charge", "collection", "death", "defense", "delay",
	"depletion", "divinity", "doom", "dread", "echo", "egg", "energy", "fate", "fetch", "fungus",
	"fuse", "growth", "hope", "ice", "judgment", "ki", "level", "lore", "loyalty", "luck", "magnet",
	"net", "oil", "omen", "page", "phyresis", "plot", "poison", "quest", "scream", "shield", "slime",
	"soul", "stun", "tide", "time", "vitality",
}
