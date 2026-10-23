// Package dotreg implements an experimental parser for the configuration
// notation described in ../../docs/config-format.md.
//
// Input starts with the required @dotReg 1 property declaration and a blank
// separator. Declarations produce no entries; entry line numbers still count
// physical lines from the beginning of the file.
//
// Parse and ParseEach resolve key paths and preserve raw right-hand sides,
// including multiline bodies with their syntax margins removed. Entry.Form
// distinguishes escaped triple-quoted bodies from literal pipe blocks.
// Repeated keys retain all value occurrences in source order, including
// repeated identical values; headings alone do not create value occurrences.
// DecodeString performs common string decoding. Build applies an
// application-defined Profile independently to each occurrence. Config offers
// collection and indexed typed lookup; singular lookup rejects multiple values.
// Each occurrence retains its own unset state and type declaration.
//
// Standard type names and the stable public API are not yet defined.
// See this sample module's README.md for supported syntax and
// provisional implementation choices.
package dotreg
