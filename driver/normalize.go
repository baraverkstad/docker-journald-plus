package driver

import "regexp"

var whitespaceRe = regexp.MustCompile(`\t[ \t]*| [ \t]+`)

// NormalizeWhitespace collapses runs of tabs or repeated spaces to a single space.
func NormalizeWhitespace(line []byte) []byte {
	return whitespaceRe.ReplaceAll(line, []byte(" "))
}
