// Package domain holds the data model and the rules that act on it.
//
// Nothing here performs I/O, and nothing here can fail: every mapper and
// predicate is total. A malformed date or an absent field yields an empty
// string, never an error — which is what makes "a bad date never drops a
// record" true by construction. Deciding to *skip* a record belongs to the
// pipeline; this package only says what the data means.
package domain

import (
	"strconv"
	"strings"
	"time"
)

// dateLayout is both the only accepted input date format and the output format.
// Go's zero-padded layout verbs require exactly two digits, so "1910-9-1" is
// rejected alongside every other shape.
const dateLayout = "2006-01-02"

// colorSeparator joins the colors array into a single CSV field.
const colorSeparator = "|"

// The only two championships that are exported. Compared case-sensitively.
const (
	serieA = "SERIE A"
	serieB = "SERIE B"
)

// Club is one input line. Keys absent from SPEC.md (titles, and the players'
// nationality and market_value) have no fields here; encoding/json drops them.
type Club struct {
	ClubID       string   `json:"club_id"`
	Name         string   `json:"name"`
	Championship string   `json:"championship"`
	FoundingDate string   `json:"founding_date"`
	City         string   `json:"city"`
	State        string   `json:"state"`
	Country      string   `json:"country"`
	Stadium      string   `json:"stadium"`
	President    string   `json:"president"`
	Nickname     string   `json:"nickname"`
	Colors       []string `json:"colors"`
	Players      []Player `json:"players"`
}

// Player is one entry of a club's players array.
//
// Age, Goals and ShirtNumber are pointers so that "absent" and "zero" stay
// distinguishable: a plain int would turn missing data into a fabricated 0.
type Player struct {
	PlayerID    string `json:"player_id"`
	Name        string `json:"name"`
	Age         *int   `json:"age"`
	Goals       *int   `json:"goals"`
	DebutDate   string `json:"debut_date"`
	Position    string `json:"position"`
	ShirtNumber *int   `json:"shirt_number"`
}

// Normalize trims the fields that carry meaning beyond their text: the two
// identifiers, which are keys, and the championship, which feeds the filter.
// After it runs, the trimmed value IS the value — no other code trims, and
// every predicate, guard and mapper downstream reads these fields directly.
// Every other field is written to CSV verbatim.
//
// Must be called exactly once per club, immediately after unmarshal and
// before any guard, filter or mapper.
//
// Normalize never judges: it does not drop anything, and an identifier that is
// blank stays blank for HasID and IsExportable to rule on.
func (c *Club) Normalize() {
	c.ClubID = strings.TrimSpace(c.ClubID)
	c.Championship = strings.TrimSpace(c.Championship)
	for i := range c.Players {
		c.Players[i].PlayerID = strings.TrimSpace(c.Players[i].PlayerID)
	}
}

// IsExportable reports whether the club passes the championship filter. It
// covers that filter *only* and says nothing about club_id — see HasID.
//
// A false result means skip the club and its players SILENTLY: an excluded
// club is expected output, not an error.
func (c *Club) IsExportable() bool {
	return c.Championship == serieA || c.Championship == serieB
}

// HasID reports whether the club has a usable key. Without one there is no
// primary key for its row and no foreign key for its players.
//
// A false result means skip the club and its players AND log it — unlike the
// filter, this is data we wanted and could not use.
func (c *Club) HasID() bool {
	return c.ClubID != ""
}

// IsExportable reports whether the player has a usable identifier.
//
// Note the asymmetry with Club.IsExportable: a false result here means skip
// this player AND log it. Same method name, opposite logging obligation.
func (p *Player) IsExportable() bool {
	return p.PlayerID != ""
}

// ClubHeader returns the clubs.csv header, in SPEC.md order.
func ClubHeader() []string {
	return []string{
		"Id do Clube",
		"Nome",
		"Campeonato",
		"Data de Fundação",
		"Cidade",
		"Estado",
		"País",
		"Estádio",
		"Presidente",
		"Apelido",
		"Cores",
	}
}

// PlayerHeader returns the players.csv header, in SPEC.md order.
func PlayerHeader() []string {
	return []string{
		"Id do Clube",
		"Id do Jogador",
		"Nome",
		"Idade",
		"Gols",
		"Data de Estreia",
		"Posição",
		"Número da Camisa",
	}
}

// Record maps the club onto a clubs.csv row. It cannot fail: unusable values
// become empty fields and the row is still produced.
func (c *Club) Record() []string {
	return []string{
		c.ClubID,
		c.Name,
		c.Championship,
		normalizeDate(c.FoundingDate),
		c.City,
		c.State,
		c.Country,
		c.Stadium,
		c.President,
		c.Nickname,
		joinColors(c.Colors),
	}
}

// Record maps the player onto a players.csv row. The club's id arrives as an
// argument — that is where the 1:N foreign key is injected, which keeps Player
// unaware of its parent.
func (p *Player) Record(clubID string) []string {
	return []string{
		clubID,
		p.PlayerID,
		p.Name,
		itoa(p.Age),
		itoa(p.Goals),
		normalizeDate(p.DebutDate),
		p.Position,
		itoa(p.ShirtNumber),
	}
}

// normalizeDate renders raw as yyyy-MM-dd, or "" if it is absent or not a
// valid date in exactly that format. It never reports an error: an unparseable
// date empties the field, it does not drop the row.
func normalizeDate(raw string) string {
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		return ""
	}
	return t.Format(dateLayout)
}

// joinColors flattens the colors array into one field, order preserved.
// A nil or empty slice yields an empty field.
func joinColors(colors []string) string {
	return strings.Join(colors, colorSeparator)
}

// itoa renders an optional number. A nil pointer is an absent value and yields
// an empty field; a real zero yields "0". The two are never conflated.
func itoa(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}
