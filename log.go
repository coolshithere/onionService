package onionService
import "fmt"

// DebugEnabled returns true if there is a DebugWriter.
func (t *onionSerivce) DebugEnabled() bool {
	return t.DebugWriter != nil
}

// Debugf writes the formatted string with a newline appended to the DebugWriter
// if present.
func (t *onionSerivce) Debugf(format string, args ...interface{}) {
	if w := t.DebugWriter; w != nil {
		fmt.Fprintf(w, format+"\n", args...)
	}
}
