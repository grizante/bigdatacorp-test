package domain

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name             string
		club             Club
		wantClubID       string
		wantChampionship string
		wantPlayerIDs    []string
	}{
		{
			name:             "already clean is untouched",
			club:             Club{ClubID: "SCCP", Championship: "SERIE A"},
			wantClubID:       "SCCP",
			wantChampionship: "SERIE A",
		},
		{
			name:             "surrounding spaces are trimmed",
			club:             Club{ClubID: "  SCCP  ", Championship: " SERIE A "},
			wantClubID:       "SCCP",
			wantChampionship: "SERIE A",
		},
		{
			name:             "tabs and newlines are trimmed",
			club:             Club{ClubID: "\tSCCP\n", Championship: "\tSERIE B\n"},
			wantClubID:       "SCCP",
			wantChampionship: "SERIE B",
		},
		{
			name:             "whitespace-only collapses to empty",
			club:             Club{ClubID: "   ", Championship: "  "},
			wantClubID:       "",
			wantChampionship: "",
		},
		{
			name: "player ids are trimmed in place",
			club: Club{
				ClubID: "SCCP",
				Players: []Player{
					{PlayerID: " SCCP-1 "},
					{PlayerID: "SCCP-2"},
					{PlayerID: "\t"},
				},
			},
			wantClubID:    "SCCP",
			wantPlayerIDs: []string{"SCCP-1", "SCCP-2", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			club := tt.club
			club.Normalize()

			if club.ClubID != tt.wantClubID {
				t.Errorf("ClubID = %q, want %q", club.ClubID, tt.wantClubID)
			}
			if club.Championship != tt.wantChampionship {
				t.Errorf("Championship = %q, want %q", club.Championship, tt.wantChampionship)
			}
			if tt.wantPlayerIDs != nil {
				got := make([]string, len(club.Players))
				for i := range club.Players {
					got[i] = club.Players[i].PlayerID
				}
				if !reflect.DeepEqual(got, tt.wantPlayerIDs) {
					t.Errorf("player ids = %q, want %q", got, tt.wantPlayerIDs)
				}
			}
		})
	}
}

// Normalize touches the three fields that carry meaning beyond their text and
// nothing else: everything else reaches the CSV verbatim, whitespace included.
func TestNormalizeLeavesOtherFieldsVerbatim(t *testing.T) {
	club := Club{
		Name:         "  Santos  ",
		FoundingDate: " 1912-04-14 ",
		City:         " Santos ",
		President:    "  Marcelo Teixeira  ",
		Nickname:     "  Peixe  ",
		Players:      []Player{{Name: "  Guilherme  ", Position: " Atacante "}},
	}
	want := club
	club.Normalize()

	if club.Name != want.Name {
		t.Errorf("Name = %q, want %q", club.Name, want.Name)
	}
	if club.FoundingDate != want.FoundingDate {
		t.Errorf("FoundingDate = %q, want %q", club.FoundingDate, want.FoundingDate)
	}
	if club.City != want.City {
		t.Errorf("City = %q, want %q", club.City, want.City)
	}
	if club.President != want.President {
		t.Errorf("President = %q, want %q", club.President, want.President)
	}
	if club.Nickname != want.Nickname {
		t.Errorf("Nickname = %q, want %q", club.Nickname, want.Nickname)
	}
	if club.Players[0].Name != want.Players[0].Name {
		t.Errorf("player Name = %q, want %q", club.Players[0].Name, want.Players[0].Name)
	}
	if club.Players[0].Position != want.Players[0].Position {
		t.Errorf("player Position = %q, want %q", club.Players[0].Position, want.Players[0].Position)
	}
}

// IsExportable assumes Normalize already ran, so its inputs are trimmed.
func TestClubIsExportable(t *testing.T) {
	tests := []struct {
		championship string
		want         bool
	}{
		{"SERIE A", true},
		{"SERIE B", true},
		{"serie a", false},
		{"Serie A", false},
		{"SERIE C", false},
		{"SEM CAMPEONATO", false},
		{"", false},
		{"SERIE AB", false},
	}

	for _, tt := range tests {
		t.Run(tt.championship, func(t *testing.T) {
			club := Club{Championship: tt.championship}
			if got := club.IsExportable(); got != tt.want {
				t.Errorf("IsExportable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClubHasID(t *testing.T) {
	tests := []struct {
		name   string
		clubID string
		want   bool
	}{
		{"present", "SCCP", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			club := Club{ClubID: tt.clubID}
			if got := club.HasID(); got != tt.want {
				t.Errorf("HasID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlayerIsExportable(t *testing.T) {
	tests := []struct {
		name     string
		playerID string
		want     bool
	}{
		{"present", "SCCP-10", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := Player{PlayerID: tt.playerID}
			if got := player.IsExportable(); got != tt.want {
				t.Errorf("IsExportable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"canonical", "1910-09-01", "1910-09-01"},
		{"leap day", "2024-02-29", "2024-02-29"},
		{"absent", "", ""},
		{"brazilian order", "01/09/1910", ""},
		{"unpadded month and day", "1910-9-1", ""},
		{"unpadded day only", "1910-09-1", ""},
		{"impossible day", "1910-02-30", ""},
		{"impossible month", "1910-13-01", ""},
		{"rfc3339 timestamp", "1910-09-01T00:00:00Z", ""},
		{"trailing space", "1910-09-01 ", ""},
		{"not a date", "ontem", ""},
		{"slashes", "1910/09/01", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDate(tt.raw); got != tt.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestJoinColors(t *testing.T) {
	tests := []struct {
		name   string
		colors []string
		want   string
	}{
		{"absent", nil, ""},
		{"empty", []string{}, ""},
		{"single", []string{"preto"}, "preto"},
		{"pair keeps order", []string{"preto", "branco"}, "preto|branco"},
		{"reversed order is preserved", []string{"branco", "preto"}, "branco|preto"},
		{"three", []string{"azul", "branco", "vermelho"}, "azul|branco|vermelho"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinColors(tt.colors); got != tt.want {
				t.Errorf("joinColors(%q) = %q, want %q", tt.colors, got, tt.want)
			}
		})
	}
}

// The point of the pointer: absent and zero must not collapse onto each other.
func TestItoa(t *testing.T) {
	tests := []struct {
		name string
		n    *int
		want string
	}{
		{"absent", nil, ""},
		{"zero is a real value", new(0), "0"},
		{"positive", new(26), "26"},
		{"negative", new(-3), "-3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itoa(tt.n); got != tt.want {
				t.Errorf("itoa() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClubRecord(t *testing.T) {
	tests := []struct {
		name string
		club Club
		want []string
	}{
		{
			name: "full club",
			club: Club{
				ClubID:       "SCCP",
				Name:         "Sport Club Corinthians Paulista",
				Championship: "SERIE A",
				FoundingDate: "1910-09-01",
				City:         "São Paulo",
				State:        "SP",
				Country:      "Brasil",
				Stadium:      "Neo Química Arena",
				President:    "Augusto Melo",
				Nickname:     "Timão",
				Colors:       []string{"preto", "branco"},
			},
			want: []string{
				"SCCP", "Sport Club Corinthians Paulista", "SERIE A", "1910-09-01",
				"São Paulo", "SP", "Brasil", "Neo Química Arena", "Augusto Melo",
				"Timão", "preto|branco",
			},
		},
		{
			name: "bad date empties the field but keeps the row",
			club: Club{ClubID: "AAA", Name: "Club A", Championship: "SERIE A", FoundingDate: "01/09/1910"},
			want: []string{"AAA", "Club A", "SERIE A", "", "", "", "", "", "", "", ""},
		},
		{
			name: "every optional absent",
			club: Club{ClubID: "AAA", Championship: "SERIE B"},
			want: []string{"AAA", "", "SERIE B", "", "", "", "", "", "", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.club.Record()
			if len(got) != len(ClubHeader()) {
				t.Fatalf("Record() has %d fields, header has %d", len(got), len(ClubHeader()))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Record() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlayerRecord(t *testing.T) {
	tests := []struct {
		name   string
		clubID string
		player Player
		want   []string
	}{
		{
			name:   "full player",
			clubID: "SCCP",
			player: Player{
				PlayerID:    "SCCP-10",
				Name:        "Rodrigo Garro",
				Age:         new(26),
				Goals:       new(8),
				DebutDate:   "2024-01-18",
				Position:    "Meia",
				ShirtNumber: new(10),
			},
			want: []string{"SCCP", "SCCP-10", "Rodrigo Garro", "26", "8", "2024-01-18", "Meia", "10"},
		},
		{
			name:   "zero goals is written as zero",
			clubID: "SCCP",
			player: Player{
				PlayerID:    "SCCP-1",
				Name:        "Hugo Souza",
				Age:         new(25),
				Goals:       new(0),
				DebutDate:   "2024-07-20",
				Position:    "Goleiro",
				ShirtNumber: new(1),
			},
			want: []string{"SCCP", "SCCP-1", "Hugo Souza", "25", "0", "2024-07-20", "Goleiro", "1"},
		},
		{
			name:   "absent numbers are empty, not zero",
			clubID: "AAA",
			player: Player{PlayerID: "P1", Name: "Player One"},
			want:   []string{"AAA", "P1", "Player One", "", "", "", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.player.Record(tt.clubID)
			if len(got) != len(PlayerHeader()) {
				t.Fatalf("Record() has %d fields, header has %d", len(got), len(PlayerHeader()))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Record() = %q, want %q", got, tt.want)
			}
		})
	}
}
