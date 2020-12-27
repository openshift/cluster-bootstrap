package common

import "fmt"

// All start command printing to stdout should go through this fmt.Printf wrapper.
// The stdout of the start command should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
