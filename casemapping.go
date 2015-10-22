package state

import "strings"

// ASCIIToLower is a casemapping helper which will lowercase a rune as
// described by the "ascii" CASEMAPPING setting.
func ASCIIToLower(r rune) rune {
	// "ascii": The ASCII characters 97 to 122 (decimal) are
	// defined as the lower-case characters of ASCII 65 to 90
	// (decimal).  No other character equivalency is defined.
	if r >= 65 && r <= 90 {
		return r + 32
	}

	return r
}

// ASCIIToUpper is a casemapping helper which will uppercase a rune as
// described by the "ascii" CASEMAPPING setting.
func ASCIIToUpper(r rune) rune {
	if r >= 97 && r <= 122 {
		return r - 32
	}

	return r
}

// StrictRFC1459ToLower is a casemapping helper which will lowercase a
// rune as described by the "strict-rfc1459" CASEMAPPING setting.
func StrictRFC1459ToLower(r rune) rune {
	// "strict-rfc1459": The ASCII characters 97 to 125 (decimal) are
	// defined as the lower-case characters of ASCII 65 to 93 (decimal).
	// No other character equivalency is defined.
	if r >= 65 && r <= 93 {
		return r + 32
	}

	return r
}

// StrictRFC1459ToUpper is a casemapping helper which will uppercase a
// rune as described by the "strict-rfc1459" CASEMAPPING setting.
func StrictRFC1459ToUpper(r rune) rune {
	if r >= 97 && r <= 125 {
		return r - 32
	}

	return r
}

// RFC1459ToLower is a casemapping helper which will lowercase a rune
// as described by the "rfc1459" CASEMAPPING setting.
func RFC1459ToLower(r rune) rune {
	// "rfc1459": The ASCII characters 97 to 126 (decimal) are defined as
	// the lower-case characters of ASCII 65 to 94 (decimal).  No other
	// character equivalency is defined.
	if r >= 65 && r <= 94 {
		return r + 32
	}

	return r
}

// RFC1459ToUpper is a casemapping helper which will uppercase a rune
// as described by the "rfc1459" CASEMAPPING setting.
func RFC1459ToUpper(r rune) rune {
	if r >= 97 && r <= 126 {
		return r - 32
	}

	return r
}

// Normalize is mostly an internal function which provides a
// normalized name based on the CASEMAPPING setting given by the
// server. Generally ToLower or ToUpper should be used.
func (s *State) Normalize(name string) string {
	return s.ToLower(name)
}

// ToLower takes the given string and lower cases it based on the
// current CASEMAPPING setting given by the server.
func (s *State) ToLower(name string) string {
	switch *s.ISupport("CASEMAPPING") {
	case "ascii":
		return strings.Map(ASCIIToLower, name)
	case "strict-rfc1459":
		return strings.Map(StrictRFC1459ToLower, name)
	}

	return strings.Map(RFC1459ToLower, name)
}

// ToUpper takes the given string and upper cases it based on the
// current CASEMAPPING setting given by the server.
func (s *State) ToUpper(name string) string {
	switch *s.ISupport("CASEMAPPING") {
	case "ascii":
		return strings.Map(ASCIIToUpper, name)
	case "strict-rfc1459":
		return strings.Map(StrictRFC1459ToUpper, name)
	}

	return strings.Map(RFC1459ToUpper, name)
}
