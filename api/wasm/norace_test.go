//go:build !race && !wasm

package wasm

// raceDetector reports that the tests run under the race detector, which
// takes several times the memory that a test allocates.
const raceDetector = false
