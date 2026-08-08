package pipeline

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"bigdatacorp-test/domain"
)

// A minimal club that passes the filter, and the row it produces. Most cases
// below vary one field of it so the expected row stays easy to read.
const (
	clubA    = `{"club_id":"AAA","name":"Club A","championship":"SERIE A"}`
	clubARow = `AAA,Club A,SERIE A,,,,,,,,`
)

// csvText renders an expected file: the header, then the rows, each terminated
// by the CRLF that encoding/csv emits per RFC 4180.
func csvText(header, rows []string) string {
	var b strings.Builder
	b.WriteString(strings.Join(header, ","))
	b.WriteString("\r\n")
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\r\n")
	}
	return b.String()
}

type runCase struct {
	name        string
	input       string
	wantClubs   []string
	wantPlayers []string
	wantLog     []string // nil means the run must be silent
}

func runCases(t *testing.T, cases []runCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var clubs, players, logw bytes.Buffer

			if err := Run(strings.NewReader(tt.input), &clubs, &players, &logw); err != nil {
				t.Fatalf("Run() = %v, want nil", err)
			}
			if got, want := clubs.String(), csvText(domain.ClubHeader(), tt.wantClubs); got != want {
				t.Errorf("clubs.csv:\n got %q\nwant %q", got, want)
			}
			if got, want := players.String(), csvText(domain.PlayerHeader(), tt.wantPlayers); got != want {
				t.Errorf("players.csv:\n got %q\nwant %q", got, want)
			}
			checkLog(t, logw.String(), tt.wantLog)
		})
	}
}

// checkLog compares the diagnostics line by line. The count must match exactly
// — that is what proves a silent skip stayed silent — while each line is
// prefix-matched, because the invalid-JSON cases end in encoding/json's own
// wording, which is not part of this program's contract.
func checkLog(t *testing.T, got string, want []string) {
	t.Helper()

	var lines []string
	if trimmed := strings.TrimSuffix(got, "\n"); trimmed != "" {
		lines = strings.Split(trimmed, "\n")
	}
	if len(lines) != len(want) {
		t.Fatalf("log has %d line(s), want %d:\n%s", len(lines), len(want), got)
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("log line %d = %q, want prefix %q", i+1, lines[i], w)
		}
	}
}

func TestRunLineLevel(t *testing.T) {
	runCases(t, []runCase{
		{
			name:    "truncated json",
			input:   `{"club_id":"AAA",` + "\n",
			wantLog: []string{"line 1: invalid JSON: "},
		},
		{
			name:    "not json at all",
			input:   "not json at all\n",
			wantLog: []string{"line 1: invalid JSON: "},
		},
		{
			name:    "json array instead of object",
			input:   "[1,2,3]\n",
			wantLog: []string{"line 1: invalid JSON: "},
		},
		{
			name:      "a bad line does not stop the run",
			input:     "not json\n" + clubA + "\n",
			wantClubs: []string{clubARow},
			wantLog:   []string{"line 1: invalid JSON: "},
		},
		{
			name:  "blank and whitespace-only lines are silent",
			input: "\n   \n\t\n",
		},
		{
			name:      "blank lines still count toward the line number",
			input:     "\n\n" + "not json\n",
			wantClubs: nil,
			wantLog:   []string{"line 3: invalid JSON: "},
		},
		{
			name:      "no trailing newline on the last line",
			input:     clubA,
			wantClubs: []string{clubARow},
		},
	})
}

func TestRunClubLevel(t *testing.T) {
	runCases(t, []runCase{
		{
			name:      "serie a passes",
			input:     clubA + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "serie b passes",
			input:     `{"club_id":"BBB","name":"Club B","championship":"SERIE B"}` + "\n",
			wantClubs: []string{`BBB,Club B,SERIE B,,,,,,,,`},
		},
		{
			name:      "surrounding whitespace passes and the trimmed value is written",
			input:     `{"club_id":"  AAA  ","name":"Club A","championship":"  SERIE A  "}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:  "other championship is excluded silently",
			input: `{"club_id":"NAC","name":"Nacional","championship":"SEM CAMPEONATO"}` + "\n",
		},
		{
			name:  "lowercase championship is excluded silently",
			input: `{"club_id":"AAA","name":"Club A","championship":"serie a"}` + "\n",
		},
		{
			name:  "mixed-case championship is excluded silently",
			input: `{"club_id":"AAA","name":"Club A","championship":"Serie A"}` + "\n",
		},
		{
			name:  "absent championship is excluded silently",
			input: `{"club_id":"AAA","name":"Club A"}` + "\n",
		},
		{
			name:  "null championship is excluded silently",
			input: `{"club_id":"AAA","name":"Club A","championship":null}` + "\n",
		},
		{
			name:  "an excluded club takes its players with it, silently",
			input: `{"club_id":"NAC","name":"Nacional","championship":"SEM CAMPEONATO","players":[{"player_id":"NAC-1","name":"Someone"}]}` + "\n",
		},
		{
			name:    "absent club_id is skipped and logged",
			input:   `{"name":"Club A","championship":"SERIE A"}` + "\n",
			wantLog: []string{"line 1: skipping club: missing club_id"},
		},
		{
			name:    "null club_id is skipped and logged",
			input:   `{"club_id":null,"name":"Club A","championship":"SERIE A"}` + "\n",
			wantLog: []string{"line 1: skipping club: missing club_id"},
		},
		{
			name:    "empty club_id is skipped and logged",
			input:   `{"club_id":"","name":"Club A","championship":"SERIE A"}` + "\n",
			wantLog: []string{"line 1: skipping club: missing club_id"},
		},
		{
			name:    "whitespace-only club_id is skipped and logged",
			input:   `{"club_id":"   ","name":"Club A","championship":"SERIE A"}` + "\n",
			wantLog: []string{"line 1: skipping club: missing club_id"},
		},
		{
			name:    "a club without an id takes its players with it",
			input:   `{"championship":"SERIE A","players":[{"player_id":"P1","name":"Player One"}]}` + "\n",
			wantLog: []string{"line 1: skipping club: missing club_id"},
		},
		{
			// The filter runs first and wins: a club we were never going to
			// export produces no diagnostics, however broken the rest of it is.
			name:  "missing club_id AND excluded championship stays silent",
			input: `{"club_id":"","name":"Club A","championship":"SEM CAMPEONATO"}` + "\n",
		},
		{
			name:  "blank club_id AND lowercase championship stays silent",
			input: `{"club_id":"   ","name":"Club A","championship":"serie a"}` + "\n",
		},
		{
			name:      "absent players yields a club row and nothing else",
			input:     clubA + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "empty players array yields a club row and nothing else",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[]}` + "\n",
			wantClubs: []string{clubARow},
		},
	})
}

func TestRunPlayerLevel(t *testing.T) {
	runCases(t, []runCase{
		{
			name:      "players carry their club id",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One"},{"player_id":"P2","name":"Player Two"}]}` + "\n",
			wantClubs: []string{clubARow},
			wantPlayers: []string{
				`AAA,P1,Player One,,,,,`,
				`AAA,P2,Player Two,,,,,`,
			},
		},
		{
			name:        "a player without an id is skipped, its siblings are kept",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One"},{"name":"No Id"},{"player_id":"P3","name":"Player Three"}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,,,,,`, `AAA,P3,Player Three,,,,,`},
			wantLog:     []string{"line 1: skipping player 2 of club AAA: missing player_id"},
		},
		{
			name:        "null, empty and blank player ids are all missing",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":null},{"player_id":""},{"player_id":"  "},{"player_id":"P4","name":"Player Four"}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P4,Player Four,,,,,`},
			wantLog: []string{
				"line 1: skipping player 1 of club AAA: missing player_id",
				"line 1: skipping player 2 of club AAA: missing player_id",
				"line 1: skipping player 3 of club AAA: missing player_id",
			},
		},
		{
			name:      "a club whose players are all unusable keeps its own row",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"name":"No Id"}]}` + "\n",
			wantClubs: []string{clubARow},
			wantLog:   []string{"line 1: skipping player 1 of club AAA: missing player_id"},
		},
		{
			name:        "the club id written to players matches the trimmed key",
			input:       `{"club_id":" AAA ","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One"}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,,,,,`},
		},
	})
}

// Every case here keeps its row: only the field is affected.
func TestRunFieldLevel(t *testing.T) {
	runCases(t, []runCase{
		{
			name:      "absent nickname is empty",
			input:     clubA + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "null nickname is empty",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","nickname":null}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "empty-string nickname is empty",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","nickname":""}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "absent colors is empty",
			input:     clubA + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "empty colors array is empty",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","colors":[]}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "colors are pipe-joined in order and never quoted",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","colors":["preto","branco"]}` + "\n",
			wantClubs: []string{`AAA,Club A,SERIE A,,,,,,,,preto|branco`},
		},
		{
			name:      "valid founding date passes through",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":"1910-09-01"}` + "\n",
			wantClubs: []string{`AAA,Club A,SERIE A,1910-09-01,,,,,,,`},
		},
		{
			name:      "null founding date empties the field, row stays",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":null}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "slashed founding date empties the field, row stays",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":"01/09/1910"}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "unpadded founding date empties the field, row stays",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":"1910-9-1"}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "impossible founding date empties the field, row stays",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":"1910-02-30"}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:      "rfc3339 founding date empties the field, row stays",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","founding_date":"1910-09-01T00:00:00Z"}` + "\n",
			wantClubs: []string{clubARow},
		},
		{
			name:        "bad debut date empties the field, player row stays",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One","debut_date":"ontem"}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,,,,,`},
		},
		{
			name:        "zero age, goals and shirt number are written as zero",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One","age":0,"goals":0,"shirt_number":0}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,0,0,,,0`},
		},
		{
			name:        "absent numbers are empty, not zero",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One"}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,,,,,`},
		},
		{
			name:        "null numbers are empty, not zero",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One","age":null,"goals":null,"shirt_number":null}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,,,,,`},
		},
		{
			name:        "a real number is written",
			input:       `{"club_id":"AAA","name":"Club A","championship":"SERIE A","players":[{"player_id":"P1","name":"Player One","age":26,"goals":8,"shirt_number":10}]}` + "\n",
			wantClubs:   []string{clubARow},
			wantPlayers: []string{`AAA,P1,Player One,26,8,,,10`},
		},
		{
			name:      "a field containing a comma is quoted",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","president":"Pedro Lourenço, Filho"}` + "\n",
			wantClubs: []string{`AAA,Club A,SERIE A,,,,,,"Pedro Lourenço, Filho",,`},
		},
		{
			name:      "a field containing a quote has it doubled",
			input:     `{"club_id":"AAA","name":"Club \"A\"","championship":"SERIE A"}` + "\n",
			wantClubs: []string{`AAA,"Club ""A""",SERIE A,,,,,,,,`},
		},
		{
			name:      "unlisted input fields are ignored",
			input:     `{"club_id":"AAA","name":"Club A","championship":"SERIE A","titles":30,"stadium_capacity":49000}` + "\n",
			wantClubs: []string{clubARow},
		},
	})
}

// Both files are valid CSV even when nothing survives, so a downstream reader
// can tell "ran, found nothing" from "never ran".
func TestRunAlwaysWritesHeaders(t *testing.T) {
	runCases(t, []runCase{
		{name: "empty input", input: ""},
		{
			name:    "every line unusable",
			input:   "not json\n" + `{"championship":"SERIE A"}` + "\n",
			wantLog: []string{"line 1: invalid JSON: ", "line 2: skipping club: missing club_id"},
		},
		{name: "everything filtered out", input: `{"club_id":"NAC","championship":"SEM CAMPEONATO"}` + "\n"},
	})
}

// A line past the scanner's cap ends the run for good. The rows accepted before
// it must still reach the caller, and the failure must still be reported.
func TestRunFlushesBeforeReportingReadFailure(t *testing.T) {
	huge := `{"club_id":"BBB","name":"` + strings.Repeat("x", 2*maxLineBytes) + `","championship":"SERIE A"}`

	var clubs, players, logw bytes.Buffer
	err := Run(strings.NewReader(clubA+"\n"+huge+"\n"), &clubs, &players, &logw)

	if err == nil {
		t.Fatal("Run() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "read failed at line 2") {
		t.Errorf("Run() = %v, want it to name line 2", err)
	}
	if got, want := clubs.String(), csvText(domain.ClubHeader(), []string{clubARow}); got != want {
		t.Errorf("rows accepted before the failure were lost:\n got %q\nwant %q", got, want)
	}
}

// The acceptance case from EXAMPLES.md, end to end over the real fixture.
func TestRunSampleFile(t *testing.T) {
	const (
		wantClubs = "Id do Clube,Nome,Campeonato,Data de Fundação,Cidade,Estado,País,Estádio,Presidente,Apelido,Cores\r\n" +
			"SCCP,Sport Club Corinthians Paulista,SERIE A,1910-09-01,São Paulo,SP,Brasil,Neo Química Arena,Augusto Melo,Timão,preto|branco\r\n" +
			"SEP,Sociedade Esportiva Palmeiras,SERIE A,1914-08-26,São Paulo,SP,Brasil,Allianz Parque,Leila Pereira,Verdão,verde|branco\r\n" +
			"SFC,Santos Futebol Clube,SERIE B,1912-04-14,Santos,SP,Brasil,Vila Belmiro,Marcelo Teixeira,,preto|branco\r\n" +
			"CRU,Cruzeiro Esporte Clube,SERIE A,1921-01-02,Belo Horizonte,MG,Brasil,Mineirão,\"Pedro Lourenço, Filho\",Raposa,azul|branco\r\n" +
			"AVA,Avaí Futebol Clube,SERIE B,1923-09-01,Florianópolis,SC,Brasil,Ressacada,Júlio Heerdt,Leão da Ilha,azul|branco\r\n"

		wantPlayers = "Id do Clube,Id do Jogador,Nome,Idade,Gols,Data de Estreia,Posição,Número da Camisa\r\n" +
			"SCCP,SCCP-10,Rodrigo Garro,26,8,2024-01-18,Meia,10\r\n" +
			"SCCP,SCCP-9,Yuri Alberto,24,15,2023-08-05,Atacante,9\r\n" +
			"SCCP,SCCP-1,Hugo Souza,25,0,2024-07-20,Goleiro,1\r\n" +
			"SEP,SEP-23,Raphael Veiga,29,10,2017-01-10,Meia,23\r\n" +
			"SEP,SEP-41,Estêvão,17,12,2024-04-01,Atacante,41\r\n" +
			"SFC,SFC-11,Guilherme,23,9,2023-02-11,Atacante,11\r\n" +
			"SFC,SFC-5,João Schmidt,31,2,2023-01-15,Volante,5\r\n" +
			"CRU,CRU-10,Matheus Pereira,28,11,2024-01-05,Meia,10\r\n"
	)

	in, err := os.Open("../sample_clubes.jsonl")
	if err != nil {
		t.Fatalf("opening the fixture: %v", err)
	}
	defer in.Close()

	var clubs, players, logw bytes.Buffer
	if err := Run(in, &clubs, &players, &logw); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if got := clubs.String(); got != wantClubs {
		t.Errorf("clubs.csv:\n got %q\nwant %q", got, wantClubs)
	}
	if got := players.String(); got != wantPlayers {
		t.Errorf("players.csv:\n got %q\nwant %q", got, wantPlayers)
	}
	// NAC is filtered out and AVA has no players: neither is an error.
	checkLog(t, logw.String(), nil)

	if got, want := strings.Count(clubs.String(), "\r\n")-1, 5; got != want {
		t.Errorf("clubs.csv has %d rows, want %d", got, want)
	}
	if got, want := strings.Count(players.String(), "\r\n")-1, 8; got != want {
		t.Errorf("players.csv has %d rows, want %d", got, want)
	}
}
