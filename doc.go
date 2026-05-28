// Package shapeshifter adapts JSON HTTP request and response contracts around
// canonical controller shapes.
//
// Applications load a versioned ShapeShifter spec at startup, create an Engine,
// and mount a framework adapter. Controllers continue to read and write their
// internal JSON shape while ShapeShifter validates, maps, coerces, and optionally
// handles external contract versions at the middleware boundary.
package shapeshifter
