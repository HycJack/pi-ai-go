// Package overflow provides context overflow detection for LLM responses.
package overflow

import (
	"regexp"
	"strings"
)

// overflowPattern matches known provider error messages indicating context overflow.
var overflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)context.*(?:length|window|limit|size).*exceed`),
	regexp.MustCompile(`(?i)maximum.*(?:context|token|length).*exceed`),
	regexp.MustCompile(`(?i)prompt.*(?:too long|exceeds|overflow)`),
	regexp.MustCompile(`(?i)input.*(?:too long|exceeds).*token`),
	regexp.MustCompile(`(?i)token.*limit.*exceed`),
	regexp.MustCompile(`(?i)this.*(?:request|model).*exceed.*(?:context|token)`),
	regexp.MustCompile(`(?i)conversation.*(?:too long|exceeds)`),
	regexp.MustCompile(`(?i)message.*(?:too long|exceeds).*token`),
	regexp.MustCompile(`(?i)context_length_exceeded`),
	regexp.MustCompile(`(?i)max.*(?:context|input).*token.*exceed`),
	regexp.MustCompile(`(?i)reduce.*(?:context|prompt|message).*length`),
	regexp.MustCompile(`(?i)exceed.*(?:context|token).*(?:limit|window|length)`),
	regexp.MustCompile(`(?i)too many tokens`),
	regexp.MustCompile(`(?i)request.*(?:too large|exceeds)`),
	regexp.MustCompile(`(?i)content.*(?:too long|exceeds)`),
	regexp.MustCompile(`(?i)total.*token.*exceed`),
	regexp.MustCompile(`(?i)input.*(?:length|size).*(?:exceed|too)`),
	regexp.MustCompile(`(?i)history.*(?:too long|exceeds)`),
	regexp.MustCompile(`(?i)sequence.*(?:too long|exceeds).*token`),
	regexp.MustCompile(`(?i)context.*overflow`),
}

// IsOverflow detects whether an error message or usage indicates context overflow.
// If contextWindow is 0, only error-based detection is used.
func IsOverflow(errMsg string, contextWindow int, usage int) bool {
	if errMsg != "" {
		lower := strings.ToLower(errMsg)
		for _, p := range overflowPatterns {
			if p.MatchString(lower) {
				return true
			}
		}
	}

	// Silent overflow: usage exceeds context window
	if contextWindow > 0 && usage > contextWindow {
		return true
	}

	return false
}

// GetPatterns returns the overflow patterns for testing.
func GetPatterns() []*regexp.Regexp {
	return overflowPatterns
}
