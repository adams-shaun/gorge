// Package forge reads Forge card-script text into the typed card IR
// (engine/cardc/ir). Each reader is a pure function of the text: it returns an
// ir node that passes ir's validator, or a *Refusal naming the first token it
// cannot read. It never guesses: a word outside its tables is a refusal.
package forge
