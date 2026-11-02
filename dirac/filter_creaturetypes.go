package dirac

import "strings"

// creatureTypeWords are the creature types of CR 205.3m as single tokens, the way a
// card script spells them. A subtype outside the list is a land, artifact, enchantment,
// spell, planeswalker, battle or plane type, which an all-creature-types object lacks.
var creatureTypeWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`
Advisor Aetherborn Alien Ally Andorian Angel Antelope Ape Archer Archon Armadillo
Armored Army Artificer Assassin Assembly-Worker Astartes Atog Attendee Aurochs
Avatar Azra B.O.B. Badger Bahamut Balloon Barbarian Bard Basilisk Bat Bear Beast
Beaver Beeble Beholder Berserker Bird Bison Blinkmoth Boar Borg Brainiac Bringer
Brushwagg C'tan Caitian Camarid Camel Capybara Caribou Carrier Cat Centaur Chef
Child Chimera Citizen Clamfolk Cleric Clown Cockatrice Construct Cow Coward Coyote
Crab Crocodile Custodes Cyberman Cyborg Cyclops Dalek Dauthi Demigod Demon Deserter
Detective Devil Dinosaur Djinn Doctor Dog Dragon Drake Dreadnought Drix Drone Druid
Dryad Duck Dwarf Echidna Efreet Egg Elder Eldrazi Elemental Elephant Elf Elk
Employee Eternal Eye Faerie Ferret Fighter Fish Flagbearer Fox Fractal Frog Fungus
Gamer Gamma Gargoyle Germ Giant Giraffe Gith Glass Glimmer Gnoll Gnome Goat Goblin
God Golem Gorgon Gorilla Gorn Graveborn Gremlin Griffin Guest Hag Halfling Hamster
Harpy Hedgehog Hellion Hero Hippo Hippogriff Homarid Homunculus Horror Horse
Horsehead Human Hydra Hyena Illusion Imp Incarnation Inhuman Inkling Inquisitor
Insect Jackal Jellyfish Judge Juggernaut Kangaroo Kavu Kelpien Killbot Kirin Kithkin
Klingon Knight Kobold Kor Kraken Kree Lamia Lammasu Lanthanite Leech Lemur Leviathan
Lhurgoyf Licid Lizard Llama Lobster Lord Mammoth Manticore Masticore Mercenary
Merfolk Metathran Minion Minotaur Mite Mole Monger Mongoose Monk Monkey Moogle
Moonfolk Mount Mouse Mutant Myr Mystic Nautilus Necron Nephilim Nightmare
Nightstalker Ninja Noble Noggle Nomad Nymph Octopus Officer Ogre Ooze Orb Orc Orgg
Orion Otter Ouphe Ox Oyster Pangolin Peasant Pegasus Pentavite Performer Pest
Phelddagrif Phoenix Phyrexian Pilot Pincher Pirate Plant Platypus Porcupine Possum
Praetor Primarch Prism Processor Q Qu Rabbit Raccoon Ranger Rat Rebel Reflection
Rhino Rigger Robot Rogue Romulan Sable Salamander Samurai Sand Saproling Satyr
Scarecrow Scientist Scion Scorpion Scout Sculpture Seal Serf Serpent Servo Shade
Shaman Shapeshifter Shark Sheep Shi'ar Siren Skeleton Skrull Skunk Slith Sliver
Sloth Slug Snail Snake Soldier Soltari Sorcerer Spawn Specter Spellshaper Sphinx
Spider Spike Spirit Splinter Sponge Spy Squid Squirrel Stained Starfish Surrakar
Survivor Symbiote Synth Talosian Teddy Tellarite Tentacle Tetravite Thalakos Tholian
Thopter Thrull Tiefling Time Tosk Toy Treefolk Trilobite Triskelavite Troll Turtle
Tyranid Unicorn Utrom Vampire Varmint Vedalken Villain Volver Vorta Vulcan Wall
Walrus Warlock Warrior Weasel Weird Werewolf Whale Wizard Wolf Wolverine Wombat Worm
Wraith Wurm Xindi Yeti Zombie Zubera
`) {
		m[w] = true
	}
	return m
}()
