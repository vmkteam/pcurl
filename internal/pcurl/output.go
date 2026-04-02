package pcurl

import (
	"fmt"
	"io"
	"strings"
)

// Output collects lines and prints them all at once.
type Output struct {
	lines []string
}

func (o *Output) Addf(format string, args ...any) {
	o.lines = append(o.lines, fmt.Sprintf(format, args...))
}

func (o *Output) Empty() {
	o.lines = append(o.lines, "")
}

func (o *Output) Print(w io.Writer) {
	for _, l := range o.lines {
		fmt.Fprintln(w, l)
	}
}

func (o *Output) String() string {
	return strings.Join(o.lines, "\n")
}
