## Input (sample_clubes.jsonl)

```
{"club_id":"SCCP","name":"Sport Club Corinthians Paulista","championship":"SERIE A","founding_date":"1910-09-01","city":"São Paulo","state":"SP","country":"Brasil","stadium":"Neo Química Arena","president":"Augusto Melo","nickname":"Timão","colors":["preto","branco"],"titles":30,"players":[{"player_id":"SCCP-10","name":"Rodrigo Garro","age":26,"goals":8,"debut_date":"2024-01-18","position":"Meia","shirt_number":10,"nationality":"Argentina","market_value":12000000},{"player_id":"SCCP-9","name":"Yuri Alberto","age":24,"goals":15,"debut_date":"2023-08-05","position":"Atacante","shirt_number":9,"nationality":"Brasil","market_value":15000000},{"player_id":"SCCP-1","name":"Hugo Souza","age":25,"goals":0,"debut_date":"2024-07-20","position":"Goleiro","shirt_number":1,"nationality":"Brasil","market_value":8000000}]}
{"club_id":"SEP","name":"Sociedade Esportiva Palmeiras","championship":"SERIE A","founding_date":"1914-08-26","city":"São Paulo","state":"SP","country":"Brasil","stadium":"Allianz Parque","president":"Leila Pereira","nickname":"Verdão","colors":["verde","branco"],"titles":26,"players":[{"player_id":"SEP-23","name":"Raphael Veiga","age":29,"goals":10,"debut_date":"2017-01-10","position":"Meia","shirt_number":23,"nationality":"Brasil","market_value":14000000},{"player_id":"SEP-41","name":"Estêvão","age":17,"goals":12,"debut_date":"2024-04-01","position":"Atacante","shirt_number":41,"nationality":"Brasil","market_value":45000000}]}
{"club_id":"SFC","name":"Santos Futebol Clube","championship":"SERIE B","founding_date":"1912-04-14","city":"Santos","state":"SP","country":"Brasil","stadium":"Vila Belmiro","president":"Marcelo Teixeira","nickname":null,"colors":["preto","branco"],"titles":8,"players":[{"player_id":"SFC-11","name":"Guilherme","age":23,"goals":9,"debut_date":"2023-02-11","position":"Atacante","shirt_number":11,"nationality":"Brasil","market_value":6000000},{"player_id":"SFC-5","name":"João Schmidt","age":31,"goals":2,"debut_date":"2023-01-15","position":"Volante","shirt_number":5,"nationality":"Brasil","market_value":3000000}]}
{"club_id":"CRU","name":"Cruzeiro Esporte Clube","championship":"SERIE A","founding_date":"1921-01-02","city":"Belo Horizonte","state":"MG","country":"Brasil","stadium":"Mineirão","president":"Pedro Lourenço, Filho","nickname":"Raposa","colors":["azul","branco"],"titles":4,"players":[{"player_id":"CRU-10","name":"Matheus Pereira","age":28,"goals":11,"debut_date":"2024-01-05","position":"Meia","shirt_number":10,"nationality":"Brasil","market_value":9000000}]}
{"club_id":"AVA","name":"Avaí Futebol Clube","championship":"SERIE B","founding_date":"1923-09-01","city":"Florianópolis","state":"SC","country":"Brasil","stadium":"Ressacada","president":"Júlio Heerdt","nickname":"Leão da Ilha","colors":["azul","branco"],"titles":0,"players":[]}
{"club_id":"NAC","name":"Nacional Atlético Clube","championship":"SEM CAMPEONATO","founding_date":"1919-06-04","city":"São Paulo","state":"SP","country":"Brasil","stadium":"Nicolau Alayon","president":"Antônio Carlos","nickname":"Naça","colors":["azul","branco","vermelho"],"titles":0,"players":[]}
```

## Expected output

Acceptance: **clubs.csv = 5 rows + header**, **players.csv = 8 rows + header**.
NAC is excluded (SEM CAMPEONATO). AVA passes the filter but has no players, so it
appears in clubs.csv and contributes nothing to players.csv.

Note: rows are terminated with CRLF (`\r\n`) per RFC 4180. This is **not** automatic —
`csv.Writer` defaults to a bare `\n`, so `UseCRLF` is set explicitly on both writers.
Shown below with plain newlines for readability.

### clubs.csv (5 rows + header)

```
Id do Clube,Nome,Campeonato,Data de Fundação,Cidade,Estado,País,Estádio,Presidente,Apelido,Cores
SCCP,Sport Club Corinthians Paulista,SERIE A,1910-09-01,São Paulo,SP,Brasil,Neo Química Arena,Augusto Melo,Timão,preto|branco
SEP,Sociedade Esportiva Palmeiras,SERIE A,1914-08-26,São Paulo,SP,Brasil,Allianz Parque,Leila Pereira,Verdão,verde|branco
SFC,Santos Futebol Clube,SERIE B,1912-04-14,Santos,SP,Brasil,Vila Belmiro,Marcelo Teixeira,,preto|branco
CRU,Cruzeiro Esporte Clube,SERIE A,1921-01-02,Belo Horizonte,MG,Brasil,Mineirão,"Pedro Lourenço, Filho",Raposa,azul|branco
AVA,Avaí Futebol Clube,SERIE B,1923-09-01,Florianópolis,SC,Brasil,Ressacada,Júlio Heerdt,Leão da Ilha,azul|branco
```

Two field-level cases to verify against:
- SFC's `Apelido` is an **empty field** (`,,`), not `""` — its `nickname` was JSON null.
- CRU's `Presidente` is **quoted** (`"Pedro Lourenço, Filho"`) because it contains a
  comma. This is the only quoted field; the pipe-joined `Cores` are never quoted.

### players.csv (8 rows + header)

```
Id do Clube,Id do Jogador,Nome,Idade,Gols,Data de Estreia,Posição,Número da Camisa
SCCP,SCCP-10,Rodrigo Garro,26,8,2024-01-18,Meia,10
SCCP,SCCP-9,Yuri Alberto,24,15,2023-08-05,Atacante,9
SCCP,SCCP-1,Hugo Souza,25,0,2024-07-20,Goleiro,1
SEP,SEP-23,Raphael Veiga,29,10,2017-01-10,Meia,23
SEP,SEP-41,Estêvão,17,12,2024-04-01,Atacante,41
SFC,SFC-11,Guilherme,23,9,2023-02-11,Atacante,11
SFC,SFC-5,João Schmidt,31,2,2023-01-15,Volante,5
CRU,CRU-10,Matheus Pereira,28,11,2024-01-05,Meia,10
```

Verify: Hugo Souza's `Gols` is `0` (literal zero, not empty). SFC's two players are
present because their club passed the filter. Every row carries its `club_id`.

## Edge cases

`sample_clubes.jsonl` is the happy path — it exercises none of the failure modes.
There is deliberately no fixture file for those; each case below is a row in the
table-driven tests (`pipeline/pipeline_test.go`, `domain/domain_test.go`). These
tables and those are meant to stay in sync.

### Line level

| Input line | clubs.csv | players.csv | log |
|---|---|---|---|
| `{"club_id":"X",` (truncated) | — | — | logged, with line number |
| `not json at all` | — | — | logged, with line number |
| empty or whitespace-only line | — | — | **silent** |

### Club level

Assume an otherwise valid club.

| Case | clubs.csv | players.csv | log |
|---|---|---|---|
| `championship` = `SERIE A` / `SERIE B` | row written | players written | silent |
| `championship` = `" SERIE A "` | row written, `Campeonato` is `SERIE A` | written | silent |
| `championship` = `SEM CAMPEONATO` | skipped | skipped | silent |
| `championship` = `serie a` (lowercase) | skipped | skipped | silent |
| `championship` absent or null | skipped | skipped | silent |
| `club_id` absent / null / `""` / `"   "`, championship passes | skipped | skipped | logged |
| `club_id` absent **and** championship fails | skipped | skipped | **silent** — the filter runs first and wins |
| `players` absent or `[]` | row written | no rows | silent |

That second-to-last row is the one to get right: a club we were never going to
export produces no diagnostics, however broken the rest of it is.

### Player level

| Case | players.csv | clubs.csv | log |
|---|---|---|---|
| `player_id` absent / null / `""` / `"   "` | that player skipped | club row kept | logged, with line number + club id |
| its siblings in the same club | still written | kept | — |

### Field level

The row is always written; only the field is affected.

| Field | Input | CSV output |
|---|---|---|
| nickname | absent / null / `""` | empty |
| colors | absent / `[]` | empty |
| colors | `["preto","branco"]` | `preto\|branco` |
| founding_date, debut_date | `"1910-09-01"` | `1910-09-01` |
| " | absent / null / `""` | empty |
| " | `"01/09/1910"` | empty |
| " | `"1910-9-1"` (unpadded) | empty |
| " | `"1910-02-30"` (impossible) | empty |
| " | `"1910-09-01T00:00:00Z"` | empty |
| age, goals, shirt_number | `0` | `0` |
| " | absent / null | empty |
| president | `"Pedro Lourenço, Filho"` | `"Pedro Lourenço, Filho"` (quoted) |