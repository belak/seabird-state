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
func (s *State) ISupport(name string) string {
	if v, ok := s.isupport[name]; ok {
		return v
	}

	if v, ok := isupportDefaults[name]; ok {
		return v
	}

	// TODO: This isn't technically correct
	return ""
}

// RPL_ISUPPORT
func (s *State) callback005(b *bot.Bot, m *irc.Message) {
	// Loop through all params aside from the first and last ones
	// as the first should always be the nick and the last should
	// always be "are supported by this server."
	for i := 1; i < len(m.Params)-1; i++ {
		// If the param starts with a -, we reset to the
		// default value
		if strings.HasPrefix(m.Params[i], "-") {
			param := s.Normalize(m.Params[i])
			delete(s.isupport, param[1:])
			continue
		}

		// Ensure there's SOMETHING for the second param in
		// the split
		split := strings.SplitN(m.Params[i], "=", 2)
		split[0] = s.Normalize(split[0])
		if len(split) != 2 {
			split = append(split, "")
		}

		s.isupport[split[0]] = split[1]
	}
}
