package state

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

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

	// These come from 004
	userModes map[rune]bool

	// These come from 005
	chanTypes map[rune]bool
	chanModes []map[rune]bool
	isupport  map[string]string

	// @ -> o
	prefixModes map[rune]rune
	// o -> @
	modePrefixes map[rune]rune

	Channels map[string]*ChannelState
	Users    map[string]*UserState
}

type UserState struct {
	Away     bool
	Channels map[string]bool
}

type ChannelState struct {
	Users map[string]bool
}

func (s *State) ensureChannel(name string) *ChannelState {
	normalizedName := s.Normalize(name)
	c, ok := s.Channels[normalizedName]
	if !ok {
		c = &ChannelState{
			Users: make(map[string]bool),
		}
		s.Channels[normalizedName] = c
	}

	return c
}

func (s *State) ensureUser(name string) *UserState {
	normalizedName := s.Normalize(name)
	u, ok := s.Users[normalizedName]
	if !ok {
		u = &UserState{
			Channels: make(map[string]bool),
		}
		s.Users[normalizedName] = u
	}

	return u
}

func (s *State) renameUser(oldName, newName string) {
	normalizedOld := s.Normalize(oldName)
	normalizedNew := s.Normalize(newName)

	u := s.ensureUser(oldName)
	delete(s.Users, normalizedOld)
	s.Users[normalizedNew] = u

	// Loop through any channels we know this user is in
	for cname := range u.Channels {
		c := s.Channels[cname]

		// Remove the old username
		delete(c.Users, normalizedOld)

		// Add the new one
		c.Users[normalizedNew] = true
	}
}

func (s *State) removeUser(name string) {
	u := s.ensureUser(name)

	// We can't modify this list while we're looping through it so
	// we need a copy.
	var channels []string
	for c := range u.Channels {
		channels = append(channels, c)
	}

	// Because ensureUserNotInChannel will delete the user if
	// there are no more channels left, we can ignore it here.
	for _, cname := range channels {
		s.ensureUserNotInChannel(name, cname)
	}
}

func (s *State) ensureUserInChannel(user, channel string) (*UserState, *ChannelState) {
	// TODO: Do we want to error if the user is not the bot and
	// the bot isn't in the channel? Might be good for
	// consistency.
	normalizedUser := s.Normalize(user)
	normalizedChannel := s.Normalize(channel)

	u := s.ensureUser(user)
	c := s.ensureChannel(channel)

	c.Users[normalizedUser] = true
	u.Channels[normalizedChannel] = true

	return u, c

}

func (s *State) ensureUserNotInChannel(user, channel string) {
	normalizedUser := s.Normalize(user)
	normalizedChannel := s.Normalize(channel)

	// TODO: There's a chance ensureUser will add a user just to
	// remove them. Might be good to not do this.
	u := s.ensureUser(user)
	c := s.ensureChannel(channel)

	// Delete the current user
	delete(c.Users, normalizedUser)
	delete(u.Channels, normalizedChannel)

	if len(u.Channels) < 1 {
		delete(s.Users, normalizedUser)
	}

	// If the bot is leaving the channel, we need to remove all
	// users from this channel.
	if user == s.currentNick {
		// We can't modify this list while we're looping through it so
		// we need a copy.
		var users []string
		for uname := range c.Users {
			users = append(users, uname)
		}

		for _, uname := range users {
			s.ensureUserNotInChannel(uname, channel)
		}

		// The only time we actually remove a channel is when
		// we leave it.
		delete(s.Channels, normalizedChannel)
	}
}

func (s *State) UserInChannel(user, channel string) bool {
	user = s.Normalize(user)
	channel = s.Normalize(channel)

	c, ok := s.Channels[channel]
	if !ok {
		return false
	}

	_, ok = c.Users[user]
	return ok
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
	b.BasicMux.Event("004", s.callback004) // RPL_MYINFO
	b.BasicMux.Event("005", s.callback005) // RPL_ISUPPORT

	b.BasicMux.Event("352", s.callback352) // RPL_WHOREPLY
	b.BasicMux.Event("315", s.callback315) // RPL_ENDOFWHO

	b.BasicMux.Event("353", s.callback353) // RPL_NAMES
	b.BasicMux.Event("366", s.callback366) // RPL_ENDOFNAMES

	// Make sure we get multi-prefix enabled as that improves some
	// of the user prefix handling.
	b.CapReq("multi-prefix")

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

	b.BasicMux.Event("PRIVMSG", func(b *bot.Bot, m *irc.Message) {
		fmt.Printf("%+v\n", s.Users)
		fmt.Printf("%+v\n", s.Channels)
	})

	return s, nil
}

func (s *State) debugCallback(b *bot.Bot, m *irc.Message) {
	log.Printf("%+v", m)
}

func (s *State) clear() {
	s.isupport = make(map[string]string)
	s.chanModes = []map[rune]bool{
		map[rune]bool{},
		map[rune]bool{},
		map[rune]bool{},
		map[rune]bool{},
	}
	s.chanTypes = make(map[rune]bool)
	s.userModes = make(map[rune]bool)
	s.prefixModes = make(map[rune]rune)
	s.modePrefixes = make(map[rune]rune)

	s.Channels = make(map[string]*ChannelState)
	s.Users = make(map[string]*UserState)

	// Create a bogus message to send through callback004 to set
	// some defaults we're missing.
	m := &irc.Message{
		Prefix:  &irc.Prefix{},
		Command: "004",
		Params:  []string{"", "", "", "Oiorw"},
	}
	s.callback004(nil, m)

	// Create a bogus message to send through callback005 so we
	// ensure any defaults which would have set special values
	// actually set things.
	m = &irc.Message{
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
		if !s.UserInChannel(s.currentNick, cname) {
			// TODO: Log warning
			return
		}

		// Run a WHO on the user to get the info we need
		b.Writef("WHO :%s", uname)
	}

	s.ensureUserInChannel(uname, cname)

}

func (s *State) partCallback(b *bot.Bot, m *irc.Message) {
	cname := m.Params[0]
	uname := m.Prefix.Name

	log.Printf("%s left channel %s", uname, cname)
	if uname == s.currentNick {
		log.Println("Bot has been left", cname)
	}

	s.ensureUserNotInChannel(uname, cname)
}

func (s *State) IsChannel(name string) bool {
	r, size := utf8.DecodeRuneInString(name)
	return size != 0 && s.chanTypes[r]
}

func (s *State) modeCallback(b *bot.Bot, m *irc.Message) {
	log.Printf("%+v", m)

	target := m.Params[0]
	modestring := m.Params[1]
	msgParams := m.Params[2:]

	isChannel := s.IsChannel(target)

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
		} else if isChannel {
			if ok := s.chanModes[0][v]; ok {
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
			} else if ok := s.chanModes[1][v]; ok {
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
			} else if ok := s.chanModes[2][v]; ok {
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
			} else if ok := s.chanModes[3][v]; ok {
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
		} else {
			if state == '+' {
				log.Printf("Setting user mode %s", string(v))
			} else {
				log.Printf("Unsetting user mode %s", string(v))
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

	s.removeUser(uname)
}

func (s *State) kickCallback(b *bot.Bot, m *irc.Message) {
	cname := m.Params[0]
	uname := m.Params[1]
	log.Printf("%s has been kicked from %s\n", uname, cname)
	if uname == s.currentNick {
		log.Println("Bot has been kicked from", cname)
	}

	s.ensureUserNotInChannel(uname, cname)
}

func (s *State) nickCallback(b *bot.Bot, m *irc.Message) {
	oldNick := m.Prefix.Name
	newNick := m.Params[0]
	log.Printf("%s changed nick to %s\n", oldNick, newNick)

	if oldNick == s.currentNick {
		log.Println("Updating current bot nick to", newNick)
		s.currentNick = newNick
	}

	s.renameUser(oldNick, newNick)
}

// RPL_WELCOME
func (s *State) callback001(b *bot.Bot, m *irc.Message) {
	s.currentNick = m.Params[0]
	s.clear()
}

// RPL_MYINFO
func (s *State) callback004(b *bot.Bot, m *irc.Message) {
	// NOTE: This would work for more than userModes, but we set a
	// number of other things in 005 so there's no point setting
	// them here. Plus, for compatibility, we can only get a
	// limited subset of what we can get from 005.
	s.userModes = make(map[rune]bool)

	umodes := m.Params[3]
	for _, mode := range umodes {
		s.userModes[mode] = true
	}
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
	if flags[0] == 'H' {
		log.Println("User is here")
		flags = flags[1:]
	} else if flags[0] == 'G' {
		log.Println("User is away")
		flags = flags[1:]
	}

	for _, c := range flags {
		log.Printf("User has prefix %s (%s)", string(c), string(s.prefixModes[c]))
	}
}

// RPL_ENDOFWHO
func (s *State) callback315(b *bot.Bot, m *irc.Message) {
	// :kenny.chatspike.net 315 guest #test :End of /WHO list.
	log.Printf("End of WHO for %s", m.Params[1])
}

// RPL_NAMES
func (s *State) callback353(b *bot.Bot, m *irc.Message) {
	// :hades.arpa 353 guest = #tethys :~&@%+aji &@Attila @+alyx +KindOne Argure
	channel := m.Params[2]
	for _, name := range strings.Split(m.Trailing(), " ") {
		// Trim prefix chars from the left
		user := strings.TrimLeftFunc(name, func(r rune) bool {
			_, ok := s.prefixModes[r]
			return ok
		})

		s.ensureUserInChannel(user, channel)

		// Grab just the modes from the original string
		modes := strings.TrimSuffix(name, user)

		// Loop through each of the modes
		for _, p := range modes {
			log.Printf("User %s has prefix %s (%s) in channel %s", user, string(p), string(s.prefixModes[p]), channel)
		}
	}
}

// RPL_ENDOFNAMES
func (s *State) callback366(b *bot.Bot, m *irc.Message) {
	// :hades.arpa 366 guest #tethys :End of /NAMES list.
	log.Printf("End of NAMES for %s", m.Params[1])
}
