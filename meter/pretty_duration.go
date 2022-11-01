package meter

import (
	"regexp"
	"strings"
	"time"
)

// PrettyDuration is a pretty printed version of a time.Duration value that cuts
// the unnecessary precision off from the formatted textual representation.
type PrettyDuration time.Duration

var prettyDurationRe = regexp.MustCompile(`\.[0-9]{4,}`)

// String implements the Stringer interface, allowing pretty printing of duration
// values rounded to three decimals.
func (d PrettyDuration) String() string {
	label := time.Duration(d).String()
	if match := prettyDurationRe.FindString(label); len(match) > 4 {
		label = strings.Replace(label, match, match[:4], 1)
	}
	return label
}
