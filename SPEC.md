# Program specification

Fields not listed below are ignored, even when present in the input (`titles`,
`nationality`, `market_value`). Every listed field is written verbatim except where
an observation says otherwise.

### clubs.csv

One line per club, in this order:

| Column (CSV) | JSON origin | Observation |
|---|---|---|
| Id do Clube | club_id | trimmed; absent/null/`""`/blank -> club **and all its players** are skipped |
| Nome | name | |
| Campeonato | championship | trimmed; must be exactly `SERIE A` or `SERIE B` (case-sensitive) or the club is skipped; the trimmed value is what is written |
| Data de Fundação | founding_date | `yyyy-MM-dd` only; anything else -> empty, row still written |
| Cidade | city | |
| Estado | state | |
| País | country | |
| Estádio | stadium | |
| Presidente | president | |
| Apelido | nickname | absent/null/`""` -> empty |
| Cores | colors | join with `\|`, order preserved; absent/`[]` -> empty |

### players.csv

One line per player (1:N from each club's `players` list), in this order:

| Column (CSV) | JSON origin | Observation |
|---|---|---|
| Id do Clube | club_id | foreign key linking the player to the club; the club's trimmed value |
| Id do Jogador | players[].player_id | trimmed; absent/null/`""`/blank -> **this player only** is skipped, club and siblings kept |
| Nome | players[].name | |
| Idade | players[].age | `*int` — absent/null -> empty; a literal `0` is written as `0` |
| Gols | players[].goals | `*int` — absent/null -> empty; a literal `0` is written as `0` |
| Data de Estreia | players[].debut_date | `yyyy-MM-dd` only; anything else -> empty, row still written |
| Posição | players[].position | |
| Número da Camisa | players[].shirt_number | `*int` — absent/null -> empty; a literal `0` is written as `0` |
