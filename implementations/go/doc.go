// Package dotreg implements an experimental parser for the configuration
// notation described in ../../docs/config-format.md.
//
// Parse and ParseEach resolve key paths and preserve raw right-hand sides.
// DecodeString performs common string decoding. Build applies an
// application-defined Profile, and Config provides typed lookup while
// preserving the distinction between unset values and explicit empty strings.
//
// Standard type names and the stable public API are not yet defined.
// See this sample module's README.md for supported syntax and
// provisional implementation choices.
package dotreg
