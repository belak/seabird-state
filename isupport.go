package state

import (
	"regexp"
	"strings"

	"github.com/belak/irc"
	"github.com/belak/seabird/bot"
)

// This matches things like:
// (ov)@+
var prefixRegex = regexp.MustCompile(`\(([^)]+)\)(.+)`)

// https://tools.ietf.org/html/draft-brocklesby-irc-isupport-03
//
// TODO: This needs to be finished. Anything with a ? as a comment
// wasn't deemed important enough to include the default for now.
var isupportDefaults = map[string]string{
	"CASEMAPPING": "rfc1459",
	"CHANNELLEN":  "200",
	"CHANTYPES":   "#&",
	"EXCEPTS":     "", // ?
	"IDCHAN":      "", // ?
	"INVEX":       "", // ?
	"MODES":       "3",
	"NICKLEN":     "9",
	"PREFIX":      "(ov)@+",
	"SAFELIST":    "", // ?
	"STATUSMSG":   "", // ?
	"STD":         "", // ?
	"TARGMAX":     "", // ?
}

// ISupport returns the value for the given server setting as reported
// by the server or the default.
func (s *State) ISupport(name string) *string {
	if v, ok := s.isupport[name]; ok {
		return &v
	}

	if v, ok := isupportDefaults[name]; ok {
		return &v
	}

	// If it hasn't been set and there's no default, the only
	// valid value is nil.
	return nil
}

// RPL_ISUPPORT
func (s *State) callback005(b *bot.Bot, m *irc.Message) {
	// Loop through all params aside from the first and last ones
	// as the first should always be the nick and the last should
	// always be "are supported by this server."
	for i := 1; i < len(m.Params)-1; i++ {
		// Ensure there's SOMETHING for the second param in
		// the split
		split := strings.SplitN(m.Params[i], "=", 2)
		split[0] = s.Normalize(split[0])
		if len(split) != 2 {
			split = append(split, "")
		}

		// If the param starts with a -, we reset to the
		// default value
		if strings.HasPrefix(split[0], "-") {
			delete(s.isupport, split[0][1:])
		} else {
			// Set it in a generic way before moving on to
			// the specifics
			s.isupport[split[0]] = split[1]
		}

		// Special handling of specific ISUPPORT tokens
		isupport := s.ISupport(split[0])
		if isupport == nil {
			continue
		}

		split[1] = *isupport
		switch split[0] {
		case "chanmodes":
			s.chanModes = []map[rune]bool{
				map[rune]bool{},
				map[rune]bool{},
				map[rune]bool{},
				map[rune]bool{},
			}

			modeSplit := strings.SplitN(split[1], ",", 5)
			for i := 0; i < len(modeSplit) && i < 4; i++ {
				for _, c := range modeSplit[i] {
					s.chanModes[i][c] = true
				}
			}
		case "chantypes":
			s.chanTypes = make(map[rune]bool)

			for _, c := range split[1] {
				s.chanTypes[c] = true
			}
		case "prefix":
			s.prefixModes = make(map[rune]rune)
			s.modePrefixes = make(map[rune]rune)

			prefixParts := prefixRegex.FindStringSubmatch(split[1])
			if prefixParts == nil || len(prefixParts[1]) != len(prefixParts[2]) {
				continue
			}

			for i := 0; i < len(prefixParts[1]); i++ {
				s.modePrefixes[rune(prefixParts[1][i])] = rune(prefixParts[2][i])
				s.prefixModes[rune(prefixParts[2][i])] = rune(prefixParts[1][i])
			}
		}
	}
}
