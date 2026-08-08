# Pipeline JSONL → CSV

Lê um arquivo JSONL de clubes de futebol e gera dois arquivos CSV: `clubs.csv`
(um clube por linha) e `players.csv` (um jogador por linha, ligado ao clube pelo
`club_id`).

Escrito em Go, **somente com a biblioteca padrão** — sem dependências externas e
sem concorrência. A leitura é feita linha a linha (`bufio.Scanner`), então o
consumo de memória é constante independente do tamanho do arquivo de entrada.

---

## Requisitos

- **Sem Docker:** Go 1.26 ou superior.
- **Com Docker:** apenas Docker (a compilação acontece dentro da imagem).

---

## Como executar sem Docker

O caminho do arquivo de entrada é passado como argumento na linha de comando:

```bash
go run . sample_clubes.jsonl
```

Os arquivos `clubs.csv` e `players.csv` são gravados **no diretório de trabalho
atual**, sobrescrevendo versões anteriores.

### Compilando um binário

```bash
go build -o pipeline .
./pipeline sample_clubes.jsonl
```

### Rodando os testes

```bash
go test ./...
```

Para ver caso a caso:

```bash
go test ./... -v
```

---

## Como executar com Docker

A imagem usa build em dois estágios: `golang:1.26` compila, e
`gcr.io/distroless/static-debian12:nonroot` executa. A imagem final tem
**~9 MB** e contém apenas o binário estático — sem shell, sem gerenciador de
pacotes, sem libc.

### 1. Build da imagem

```bash
docker build -t clubs-pipeline .
```

Os testes rodam **dentro do estágio de build**. Se algum teste falhar, a imagem
não é gerada.

### 2. Execução

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/data" \
    clubs-pipeline sample_clubes.jsonl
```

Depois disso, `clubs.csv` e `players.csv` aparecem no seu diretório atual.

### Entendendo cada parte do comando

| Trecho | Para que serve |
|---|---|
| `--rm` | Remove o contêiner ao terminar. |
| `--user "$(id -u):$(id -g)"` | **Obrigatório.** Veja a explicação abaixo. |
| `-v "$PWD:/data"` | Monta seu diretório atual em `/data`, que é o `WORKDIR` da imagem. É por aí que a entrada entra e a saída sai. |
| `sample_clubes.jsonl` | O argumento do programa, relativo a `/data`. Troque pelo arquivo que quiser processar. |

O argumento é repassado direto ao binário, porque o `Dockerfile` usa
`ENTRYPOINT ["/pipeline"]`. Qualquer coisa depois do nome da imagem vira
argumento do programa:

```bash
# processando outro arquivo
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/data" \
    clubs-pipeline meus_clubes.jsonl
```

### Por que `--user` é obrigatório

A imagem roda como o usuário `nonroot` (uid 65532). Esse uid **não tem permissão
de escrita** num diretório do host que pertence a você, então sem a flag a
execução falha com `cannot create clubs.csv`.

Passando `--user "$(id -u):$(id -g)"`, o contêiner assume o seu uid e os CSVs
gerados ficam com a sua propriedade — e não de `root`.

> A alternativa seria usar a variante `root` da imagem distroless, que
> dispensaria a flag. Foi descartada porque os arquivos gerados no host ficariam
> pertencendo a `root`, exigindo `sudo` só para apagá-los.

---

## Argumentos e códigos de saída

O programa aceita **exatamente um** argumento: o caminho do arquivo JSONL.

| Situação | stderr | Código de saída |
|---|---|---|
| Sucesso (mesmo com linhas descartadas) | diagnósticos das linhas descartadas | `0` |
| Argumento ausente ou em excesso | mensagem + `usage:` | `1` |
| Arquivo de entrada não pôde ser aberto | mensagem + `usage:` | `1` |
| Falha de leitura ou de escrita no meio do processo | mensagem de erro | `1` |

Repare na primeira linha: **linhas descartadas não são falha**. Um arquivo com
registros problemáticos gera saída parcial e termina com código `0`. Só um erro
de infraestrutura (arquivo inexistente, disco cheio, linha maior que o buffer)
resulta em código diferente de zero.

O arquivo de entrada é aberto **antes** dos arquivos de saída serem criados. Se
você digitar o caminho errado, os `clubs.csv` e `players.csv` de uma execução
anterior continuam intactos.

---

## O fluxo

Cada linha do JSONL é processada isoladamente, nesta ordem:

```
                 ┌─────────────────────┐
   linha do   →  │  json.Unmarshal     │ → inválido? loga e pula a linha
   arquivo       └──────────┬──────────┘
                            ↓
                 ┌─────────────────────┐
                 │  club.Normalize()   │  apara club_id, championship, player_id
                 └──────────┬──────────┘
                            ↓
                 ┌─────────────────────┐
                 │ club.IsExportable() │ → falso? pula o clube EM SILÊNCIO
                 └──────────┬──────────┘
                            ↓
                 ┌─────────────────────┐
                 │   club.HasID()      │ → falso? loga e pula o clube + jogadores
                 └──────────┬──────────┘
                            ↓
                    grava linha em clubs.csv
                            ↓
                 para cada jogador do clube:
                 ┌─────────────────────────┐
                 │  player.IsExportable()  │ → falso? loga e pula SÓ esse jogador
                 └──────────┬──────────────┘
                            ↓
                    grava linha em players.csv
```

### Os modos de falha

Eles são tratados de formas deliberadamente diferentes:

| Modo | O que acontece | Vai para o log? |
|---|---|---|
| **JSON inválido** (a linha não faz parse) | pula a linha inteira e continua | sim, com o número da linha |
| **`club_id` ausente** | pula o clube **e todos os seus jogadores** | sim, com o número da linha |
| **`player_id` ausente** | pula **apenas aquele jogador**; o clube e os demais jogadores continuam | sim, com número da linha e id do clube |
| **Data inválida** | o campo fica vazio, **a linha continua sendo gravada** | não |
| **Campo opcional ausente/nulo** | o campo fica vazio, **a linha continua sendo gravada** | não |
| **Clube reprovado no filtro** | pula o clube e seus jogadores | **não — silêncio** |

Os dois primeiros "pulos" e o último são muito diferentes entre si: um clube fora
da Série A/B é uma **saída esperada**, não um erro. Ele nunca aparece no log.

Exemplo de execução com um arquivo problemático:

```
line 1: skipping player 2 of club AAA: missing player_id
line 2: invalid JSON: invalid character 'T' looking for beginning of object key string
line 4: skipping club: missing club_id
exit=0
```

---

## Decisões de projeto

### 1. Três pacotes, separados pelo que pode falhar

```
main.go                # argumentos, arquivos, códigos de saída. Sem regra de negócio.
domain/domain.go       # o modelo e as regras. Sem I/O.
pipeline/pipeline.go   # o fluxo de dados e as decisões de descarte.
```

### 2. `domain` não pode falhar

Nenhuma função de `domain` retorna `error`. Uma data malformada vira string
vazia; nunca vira falha.

**Por quê:** é isso que torna a regra *"uma data ruim nunca derruba um
registro"* verdadeira **por construção**, e não por disciplina de quem escreve o
código. Todo o descarte de registros mora em `pipeline`; toda a normalização
mora em `domain`. Os dois nunca se misturam.

### 3. Números opcionais são `*int` — o programa não inventa dados

`age`, `goals` e `shirt_number` são ponteiros:

| Entrada JSON | Saída no CSV |
|---|---|
| `"goals": 0` | `0` |
| `"goals": null` | *(vazio)* |
| campo ausente | *(vazio)* |

**Por quê:** com um `int` comum, um campo ausente viraria `0` depois do
unmarshal, e o programa estaria **fabricando um dado que ninguém informou**. Um
goleiro que não fez nenhum gol e um jogador cujo número de gols é desconhecido
são coisas diferentes, e o CSV precisa distinguir as duas.

O `0` nunca é tratado como ausente — mesmo quando é implausível. Um jogador com
`"age": 0` sai como `0`: isso é um problema do dado de origem, não nosso.

### 4. Datas: só `yyyy-MM-dd` é aceito

Um único `time.Parse` com o layout `2006-01-02`. Qualquer outra coisa vira campo
vazio, **sem descartar a linha**.

São rejeitados: `18/01/2024`, `2024-1-8` (sem zero à esquerda),
`2024-01-18T00:00:00Z` e datas impossíveis como `2024-02-30`.

**Por quê:** aceitar vários formatos parece generoso, mas transforma o
"conserto" em adivinhação — `01/02/2024` é 1º de fevereiro ou 2 de janeiro?
Melhor ter uma regra única e explícita.

### 5. `""` conta como identificador ausente

Para `club_id` e `player_id`, os quatro casos são equivalentes: campo ausente,
`null`, `""` e só espaços em branco.

**Por quê:** uma chave vazia é tão inútil quanto uma chave inexistente. Não há o
que fazer com ela — nem como chave primária, nem como chave estrangeira.

### 6. O filtro apara espaços, mas respeita maiúsculas e minúsculas

`" SERIE A "` passa. `"serie a"` **não** passa.

**Por quê:** espaço em volta é ruído de digitação, sem significado. Já a
diferença entre maiúsculas e minúsculas pode indicar um valor vindo de outro
sistema, e a especificação diz "exatamente `SERIE A` ou `SERIE B`".

E o valor gravado no CSV é o **aparado** (`SERIE A`), não o original — se já
decidimos que aquilo *é* Série A, não faz sentido carregar o ruído adiante.

### 7. O trim acontece em um único ponto

`strings.TrimSpace` aparece **uma única vez** em todo o código: dentro de
`(*Club).Normalize()`, que roda uma vez por linha, logo após o unmarshal e antes
de qualquer validação ou mapeamento.

```go
func (c *Club) Normalize() {
	c.ClubID = strings.TrimSpace(c.ClubID)
	c.Championship = strings.TrimSpace(c.Championship)
	for i := range c.Players {
		c.Players[i].PlayerID = strings.TrimSpace(c.Players[i].PlayerID)
	}
}
```

**Por quê:** depois que `Normalize` roda, **o valor aparado é o valor**. Nenhuma
outra função precisa lembrar de aparar nada, e o `club_id` gravado em
`players.csv` é byte a byte igual ao gravado em `clubs.csv` — o que mantém o
relacionamento 1:N íntegro.

Detalhe importante: o laço é por **índice**, não por valor. `for _, p := range`
apararia uma cópia e a jogaria fora — o clássico no-op silencioso.

`Normalize` também **não julga**: ela não sabe o que é `SERIE A` e não descarta
nada. Um `club_id` em branco continua em branco para o `HasID()` decidir.

### 8. O filtro roda antes da checagem de `club_id` — e o silêncio é incondicional

Um clube reprovado no filtro nunca gera diagnóstico, **mesmo que também esteja
sem `club_id`**.

**Por quê:** o log existe para avisar sobre dados que a gente **queria** usar e
não conseguiu. Um clube de "SEM CAMPEONATO" nunca seria exportado de qualquer
forma — reclamar do `club_id` dele seria ruído puro. Por isso a ordem das
verificações é parte do contrato, não um detalhe de implementação.

### 9. `IsExportable` é assimétrico — e por isso `HasID` existe separado

- `(*Club).IsExportable()` → **só** o filtro de campeonato. Falso significa
  *pular em silêncio*.
- `(*Club).HasID()` → tem `club_id` utilizável. Falso significa *pular e logar*.
- `(*Player).IsExportable()` → tem `player_id`. Falso significa *pular e logar*.

**Por quê:** um clube tem **dois motivos independentes** para ser descartado, e
eles têm obrigações de log opostas. Se um único booleano juntasse os dois, o
`pipeline` não teria como saber qual dos dois aconteceu e logaria a coisa errada.

Note a assimetria: no `Club`, `IsExportable` falso é silêncio; no `Player`, é
log. Mesmo nome, obrigações contrárias — está comentado no código exatamente por
isso.

### 10. `Run` recebe `io.Writer`, inclusive para o log

```go
func Run(in io.Reader, clubsOut, playersOut, logw io.Writer) error
```

**Por quê:** o `main` passa `os.Stderr`; os testes passam um `*bytes.Buffer`. Se
o log fosse fixo em `os.Stderr`, seria impossível testar a distinção mais
importante do programa — *o que é logado* versus *o que é silencioso*.

### 11. Os cabeçalhos são sempre gravados

Mesmo que nenhum registro sobreviva, os dois CSVs saem com sua linha de
cabeçalho.

**Por quê:** o resultado continua sendo um CSV válido e legível por qualquer
leitor, e distingue "rodou e não achou nada" de "não rodou".

### 12. CRLF é explícito

Os CSVs terminam as linhas com `\r\n`, conforme a RFC 4180.

**Por que vale a nota:** isso **não** é automático. O `csv.Writer` do Go escreve
`\n` puro a menos que `UseCRLF` seja ativado — o que `Run` faz nos dois writers.

### 13. Testes table-driven, sem arquivo de fixture para os casos de erro

`sample_clubes.jsonl` cobre só o caminho feliz. Todos os casos de falha (JSON
inválido, ids ausentes, datas ruins, linhas em branco) são linhas de tabela nos
testes, comparadas contra `*bytes.Buffer`.

**Por quê:** um segundo arquivo de fixture exigiria um segundo arquivo de saída
esperada, e a relação entre os dois ficaria implícita. Na tabela, entrada e
saída esperada ficam lado a lado, na mesma linha.

### 14. No Docker, os testes travam o build

O `Dockerfile` roda `go test ./...` no estágio de build, antes do `go build`.

**Por quê:** garante que nenhum binário quebrado seja empacotado. Como
consequência, o `.dockerignore` **precisa manter** o `sample_clubes.jsonl` no
contexto de build — o teste de aceitação lê esse arquivo.

---

## Estrutura de arquivos

```
main.go                    # argumentos, arquivos, códigos de saída
domain/domain.go           # structs, Normalize, predicados, mapeadores
domain/domain_test.go      # testes das regras
pipeline/pipeline.go       # o fluxo: scan, unmarshal, filtro, escrita
pipeline/pipeline_test.go  # testes de ponta a ponta sobre *bytes.Buffer
Dockerfile                 # build em dois estágios
.dockerignore              # mantém sample_clubes.jsonl (os testes precisam dele)
sample_clubes.jsonl        # entrada de exemplo
```

Documentação complementar: [SPEC.md](SPEC.md) traz o mapeamento coluna a coluna,
e [EXAMPLES.md](EXAMPLES.md) traz a saída esperada e a tabela de casos de borda.

---

## Resultado esperado com o arquivo de exemplo

`clubs.csv` com **5 linhas + cabeçalho** e `players.csv` com **8 linhas +
cabeçalho**, sem nada no stderr.

- **NAC** é excluído: campeonato `SEM CAMPEONATO`.
- **AVA** passa no filtro mas não tem jogadores — aparece em `clubs.csv` e não
  contribui com nenhuma linha em `players.csv`.
- O apelido do **SFC** sai como campo vazio, porque o `nickname` era `null`.
- O presidente do **CRU** sai entre aspas, porque contém vírgula.
- Os gols do **Hugo Souza** saem como `0` literal, não como campo vazio.
