package store

import (
	"regexp"
	"strings"
)

var (
	yearRe  = regexp.MustCompile(`[\[(]\s*([0-9]{4})\s*[\])]`)
	driveRe = regexp.MustCompile(`(?i)\b(2wd|4wd|fwd|rwd|awd)\b`)
	swRe       = regexp.MustCompile(`(?i)\bs\.w\.?\b`)
	atRe       = regexp.MustCompile(`(?i)\ba\.t\b`)
	sbackRe    = regexp.MustCompile(`(?i)\bs\.back\b`)
	quoteYrRe  = regexp.MustCompile("(?:[`'‘’])\\s*[0-9]{1,2}\\b")
	bracketRe  = regexp.MustCompile(`\[[^\]]*\]`)
	parenRe    = regexp.MustCompile(`\([^)]*\)`)
	wordRe     = regexp.MustCompile(`[^a-z0-9-]+`)
	underscore = regexp.MustCompile(`_+`)
)

// tokenOverrides sets canonical casing for tokens that would otherwise
// title-case into something that does not read like the model.
var tokenOverrides = map[string]string{
	"sw": "SW", "at": "AT",
	"ii": "II", "iii": "III",
	"gt": "GT", "gt3": "GT3", "gtb": "GTB", "gto": "GTO",
	"tt": "TT", "xj": "XJ", "xm": "XM", "xjs": "XJS",
	"mpv": "MPV", "rx7": "RX7", "rx8": "RX8",
	"4c": "4C",
	"ix35": "IX35",
	"i10": "I10", "i20": "I20", "i30": "I30", "i40": "I40",
	"x200": "X200", "x350": "X350", "x358": "X358", "x368": "X368",
	"v50": "V50", "v70": "V70", "v90": "V90",
}

// CargoYear extracts the four-digit model year from a bracket- or
// parenthesis-delimited group in a raw schematic model name
// (e.g. "accent (cm41) [2006]" -> "2006"). Model numbers like 9000 are never
// returned because a year only counts inside [...](...). Returns "".
func CargoYear(raw string) string {
	if m := yearRe.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return ""
}

// CleanDisplay turns raw folder-style model names into readable titles:
//   "coupe`  (2002)"            -> "Coupe"
//   "accent (cm41) [2006]"      -> "Accent"
//   "147 ii° serie [2004]"      -> "147 II Serie"
//   "156 [2001] s.w"            -> "156 SW"
//   "ix35 [2010] (4wd)"         -> "IX35 4WD"
//   "911 [1993]"                -> "911"
//
// The raw name is preserved as the model identity; this is display-only.
func CleanDisplay(raw string) string {
	if raw == "" {
		return ""
	}
	s := strings.ToLower(strings.TrimSpace(raw))
	s = underscore.ReplaceAllString(s, " ")

	var tags []string
	for _, t := range driveRe.FindAllString(s, -1) {
		tags = append(tags, strings.ToUpper(t))
	}
	s = driveRe.ReplaceAllString(s, " ")
	s = quoteYrRe.ReplaceAllString(s, " ")
	// datashed years always live in brackets or parentheses; dropping those
	// groups leaves 3- and 4-digit model numbers (147, 9000, 3008) intact.
	s = bracketRe.ReplaceAllString(s, " ")
	s = parenRe.ReplaceAllString(s, " ")
	s = swRe.ReplaceAllString(s, " sw ")
	s = atRe.ReplaceAllString(s, " at ")
	s = sbackRe.ReplaceAllString(s, " sportback ")

	var words []string
	for _, unit := range wordRe.Split(s, -1) {
		unit = strings.Trim(unit, "-")
		if unit == "" {
			continue
		}
		if v, ok := tokenOverrides[unit]; ok {
			words = append(words, v)
			continue
		}
		words = append(words, titleWord(unit))
	}

	joined := strings.Join(words, " ")
	for _, t := range tags {
		joined = strings.TrimSpace(joined + " " + t)
	}
	if joined == "" {
		return raw
	}
	return joined
}

func titleWord(s string) string {
	out := []rune(s)
	up := true
	for i := range out {
		if out[i] == '-' {
			up = true
			continue
		}
		if up && out[i] >= 'a' && out[i] <= 'z' {
			out[i] = out[i] - 'a' + 'A'
		}
		up = false
	}
	return string(out)
}