package state

import (
	"errors"
	"log"
	"strings"

	"github.com/belak/irc"
	"github.com/belak/seabird/bot"
)

func init() {
	bot.RegisterPlugin("state", NewStatePlugin)
}

/*
 * TODO:
 * Public interface
 * Stop making assumptions about the number of params
 */

// State is a plugin which will track the state of users and channels.
type State struct {
	currentNick string

	chanModes []string
	isupport  map[string]string

	prefixModes  map[rune]rune
	modePrefixes map[rune]rune
}

func NewStatePlugin(b *bot.Bot) (bot.Plugin, error) {
	s := &State{}
	s.clear()

	b.BasicMux.Event("JOIN", s.joinCallback)
	b.BasicMux.Event("KICK", s.kickCallback)
	b.BasicMux.Event("MODE", s.modeCallback)
	b.BasicMux.Event("NICK", s.nickCallback)
	b.BasicMux.Event("PART", s.partCallback)
	b.BasicMux.Event("QUIT", s.quitCallback)

	b.BasicMux.Event("001", s.callback001) // RPL_WELCOME
	b.BasicMux.Event("005", s.callback005) // RPL_ISUPPORT

	b.BasicMux.Event("352", s.callback352) // RPL_WHOREPLY
	b.BasicMux.Event("315", s.callback315) // RPL_ENDOFWHO

	b.BasicMux.Event("353", s.callback353) // RPL_NAMES
	b.BasicMux.Event("366", s.callback366) // RPL_ENDOFNAMES

	// TODO: CAP REQ multi-prefix

	/* These are callbacks which might be useful eventually
	b.BasicMux.Event("TOPIC", s.topicCallback)
	b.BasicMux.Event("221", s.callback221) // RPL_UMODEIS
	b.BasicMux.Event("305", s.callback305) // RPL_UNAWAY
	b.BasicMux.Event("306", s.callback306) // RPL_NOWAWAY
	b.BasicMux.Event("324", s.callback324) // RPL_CHANNELMODEIS
	b.BasicMux.Event("328", s.callback328) // RPL_CHANNEL_URL
	b.BasicMux.Event("329", s.callback329) // RPL_CREATIONTIME
	b.BasicMux.Event("332", s.callback332) // RPL_TOPIC
	b.BasicMux.Event("333", s.callback333) // RPL_TOPICWHOTIME
	b.BasicMux.Event("346", s.callback346) // RPL_INVITELIST
	b.BasicMux.Event("347", s.callback347) // RPL_ENDOFINVITELIST
	b.BasicMux.Event("348", s.callback348) // RPL_EXCEPTLIST
	b.BasicMux.Event("349", s.callback349) // RPL_ENDOFEXCEPTLIST
	b.BasicMux.Event("367", s.callback367) // RPL_BANLIST
	b.BasicMux.Event("368", s.callback368) // RPL_ENDOFBANLIST
	*/

	return s, nil
}

func (s *State) clear() {
	s.isupport = make(map[string]string)
	s.chanModes = []string{"", "", "", ""}
	s.prefixModes = make(map[rune]rune)
	s.modePrefixes = make(map[rune]rune)

	// Create a bogus message to send through callback005 so we
	// ensure any defaults which would have set special values
	// actually set things.
	m := &irc.Message{
		Prefix:  &irc.Prefix{},
		Command: "005",
		Params:  []string{},
	}
	for k := range isupportDefaults {
		m.Params = append(m.Params, "-"+k)
	}
	m.Params = append(m.Params, "are supported by this server.")

	s.callback005(nil, m)
}

func (s *State) joinCallback(b *bot.Bot, m *irc.Message) {
	cname := m.Params[0]
	uname := m.Prefix.Name

	log.Printf("%s joined channel %s\n", uname, cname)

	if uname == s.currentNick {
		log.Println("Joining new channel")

		// Queue up a WHO so we can get all the nicks in this
		// channel.
		//
		// TODO: This might not be needed if RPL_NAMES has
		// what we need.
		b.Writef("WHO :%s", cname)
	} else {
		// Run a WHO on the user to get the info we need
		b.Writef("WHO :%s", uname)
	}
}

func (s *State) partCallback(b *bot.Bot, m *irc.Message) {
	cname := m.Params[0]
	uname := m.Prefix.Name
	log.Printf("%s left channel %s", uname, cname)
	if uname == s.currentNick {
		log.Println("Bot has been left", cname)
	}
}

func (s *State) modeCallback(b *bot.Bot, m *irc.Message) {
	log.Printf("%+v", m)

	modestring := m.Params[1]
	msgParams := m.Params[2:]

	// Convenience function to modify the slice and pop the first param
	popParam := func() (string, error) {
		if len(msgParams) == 0 {
			return "", errors.New("No more params")
		}

		p := msgParams[0]
		msgParams = msgParams[1:]

		return p, nil
	}

	state := '+'

	for _, v := range modestring {
		if v == '+' || v == '-' {
			state = v
		} else if strings.IndexRune(s.chanModes[0], v) != -1 {
			// list-like (always take param)
			p, err := popParam()
			if err != nil {
				continue
			}

			if state == '+' {
				log.Printf("Adding %s to list for mode %s", p, string(v))
			} else {
				log.Printf("Removing %s from list for mode %s", p, string(v))
			}
		} else if strings.IndexRune(s.chanModes[1], v) != -1 {
			// key-like (always take param)
			p, err := popParam()
			if err != nil {
				continue
			}

			if state == '+' {
				log.Printf("Setting mode %s with param %s", string(v), p)
			} else {
				log.Printf("Unsetting mode %s with param %s", string(v), p)
			}
		} else if strings.IndexRune(s.chanModes[2], v) != -1 {
			// limit-like (take param if in + state)
			if state == '+' {
				p, err := popParam()
				if err != nil {
					continue
				}

				log.Printf("Setting mode %s to %s", string(v), p)
			} else {
				log.Printf("Unsetting mode %s", string(v))
			}
		} else if strings.IndexRune(s.chanModes[3], v) != -1 {
			// settings (never take param)
			if state == '+' {
				log.Printf("Setting mode %s", string(v))
			} else {
				log.Printf("Unsetting mode %s", string(v))
			}
		} else if mp, ok := s.modePrefixes[v]; ok {
			// user prefix (always take param)
			p, err := popParam()
			if err != nil {
				continue
			}

			if state == '+' {
				log.Printf("Setting prefix %s (%s) on user %s", string(mp), string(v), p)
			} else {
				log.Printf("Unsetting prefix %s (%s) on user %s", string(mp), string(v), p)
			}
		}
	}

}

func (s *State) quitCallback(b *bot.Bot, m *irc.Message) {
	uname := m.Prefix.Name
	log.Printf("%s has quit", uname)
	if uname == s.currentNick {
		log.Printf("Bot has quit. This is generally bad.")
		// TODO: Well, shit. At this point it probably doesn't
		// matter what we do.
	}
}

func (s *State) kickCallback(b *bot.Bot, m *irc.Message) {
	cname := m.Params[0]
	uname := m.Params[1]
	log.Printf("%s has been kicked from %s\n", uname, cname)
	if uname == s.currentNick {
		log.Println("Bot has been kicked from", cname)
	}
}

func (s *State) nickCallback(b *bot.Bot, m *irc.Message) {
	oldNick := m.Prefix.Name
	newNick := m.Params[0]
	log.Printf("%s changed nick to %s\n", oldNick, newNick)

	if oldNick == s.currentNick {
		log.Println("Updating current bot nick to", newNick)
		s.currentNick = newNick
	}
}

// RPL_WELCOME
func (s *State) callback001(b *bot.Bot, m *irc.Message) {
	s.currentNick = m.Params[0]
	s.clear()
}

// RPL_WHOREPLY
func (s *State) callback352(b *bot.Bot, m *irc.Message) {
	// <source> 352 <target> <channel> <user> <host> <server> <nick> <flags> :<distance> <realname>
	// :kenny.chatspike.net 352 guest #test grawity broken.symlink *.chatspike.net grawity H@%+ :0 Mantas M.
	var (
		// target  = m.Params[0]
		channel = m.Params[1]
		user    = m.Params[2]
		host    = m.Params[3]
		// server  = m.Params[4]
		nick  = m.Params[5]
		flags = m.Params[6]
		// rest = m.Params[7] // Or m.Trailing()
	)

	log.Printf("Flags for %s!%s@%s on %s: %s", nick, user, host, channel, flags)
}

// RPL_ENDOFWHO
func (s *State) callback315(b *bot.Bot, m *irc.Message) {
	// :kenny.chatspike.net 315 guest #test :End of /WHO list.

}

// RPL_NAMES
func (s *State) callback353(b *bot.Bot, m *irc.Message) {
	// :hades.arpa 353 guest = #tethys :~&@%+aji &@Attila @+alyx +KindOne Argure

}

// RPL_ENDOFNAMES
func (s *State) callback366(b *bot.Bot, m *irc.Message) {
	// :hades.arpa 366 guest #tethys :End of /NAMES list.
}
