# Arquitetura

parallelepiped é um app desktop (Wails v2 + WebView2) organizado como **monolito
modular em camadas**: um único executável, módulos de negócio com fronteiras
explícitas e infraestrutura transversal em `internal/platform`.

## Estrutura

```text
main.go                    composition root (Wails exige o pacote main na raiz)
migrations/                schema versionado do banco do índice (embutido no binário)
internal/
  modules/
    vault/                 abrir cofre, cofres recentes, árvore de arquivos
    notes/                 ler, salvar, criar, renomear e apagar notas (.md)
    index/                 índice SQLite: links, backlinks, tags, busca FTS5, grafo
      handler/             struct ligada ao frontend pelo Wails (transporte)
      service/             casos de uso
      model/               entidades e regras puras (parse de [[links]], grafo)
      repository/          SQL e sistema de arquivos
      dto/                 payloads com tags json
  platform/
    bridge/                converte erros no envelope enviado ao frontend
    config/                diretórios do usuário (config, cache, log)
    database/              abertura do SQLite, migrations e WithTx
    desktop/               contexto e diálogos nativos do Wails
    errors/                categorias e catálogo único de erros sentinela
    log/                   logger estruturado (slog, JSON em arquivo)
    validation/            acumulador de erros por campo
frontend/                  TypeScript + Vite, CodeMirror 6, 3d-force-graph
tools/makehelp/            imprime os alvos do Makefile (make help)
```

Dependência dentro de cada módulo, sempre numa direção:

```text
handler -> service -> repository -> SQLite / arquivos
   |          |            |
  dto       model      rows/scan
```

Módulos conversam por interfaces pequenas definidas no consumidor, com tipos
simples — nunca importando o handler ou o model de outro módulo:

- `vault/service.IndexBuilder` — o vault pede ao índice para reconstruir ao abrir um cofre.
- `notes/service.RootProvider` — as notas perguntam ao vault qual é a pasta aberta.
- `notes/service.Indexer` — cada escrita de nota atualiza o índice (`Upsert`/`Remove`).

## Adaptações da arquitetura para desktop

A skill de referência descreve backends HTTP. No desktop:

| Conceito HTTP | Aqui |
|---|---|
| handler `net/http` | struct ligada ao Wails (`Bind`); cada método exportado vira uma chamada JS |
| status HTTP por categoria | o envelope `{code, message, details}` vira o texto do erro que rejeita a promise; `frontend/src/api/client.ts` o interpreta |
| `5xx` genérico | categoria `internal`: o detalhe vai para o log, o frontend recebe mensagem genérica |
| `cmd/api/main.go` | `main.go` na raiz — o Wails compila o pacote main da raiz |
| sessão, CSRF, rate limit, IP real | não se aplicam: um usuário local, sem rede |

### Migrations aplicadas na abertura

A regra "nunca migrar no startup" existe para bancos compartilhados com deploy
supervisionado. O banco do índice é **local, de um único usuário e derivado do
cofre**: pode ser apagado e reconstruído a qualquer momento a partir dos
arquivos `.md`. Por isso `platform/database.Open` aplica as migrations
pendentes (controladas por `PRAGMA user_version`) ao abrir o banco.

Os arquivos ficam em `migrations/` na raiz, nomeados `<versão>_<descrição>.sql`,
e são embutidos pelo pacote `migrations`, que a composition root injeta no
repositório do índice.

### O arquivo é a fonte da verdade

As notas são arquivos Markdown comuns. O índice SQLite (em
`%LOCALAPPDATA%\parallelepiped\index\`) é cache: se uma atualização do índice
falhar depois que o arquivo foi gravado, a falha é registrada no log e a
gravação não é desfeita. Abrir o cofre de novo reconstrói o índice.

### Notas apagadas

Apagar uma nota a move para `.trash/` dentro do cofre, com data no nome;
nada é excluído permanentemente pelo app.

## Contrato de erro com o frontend

```json
{ "code": "invalid_input", "message": "invalid data", "details": { "path": "Informe o nome da nota." } }
```

Códigos: `invalid_input`, `not_found`, `conflict`, `unavailable`, `internal`.
`details` traz mensagens por campo já no idioma do produto (português).

## Próximas fases

1. Watcher do cofre (fsnotify) para reindexar mudanças feitas fora do app.
2. Renomear nota reescrevendo os `[[links]]` que apontam para ela.
3. Pré-visualização do Markdown renderizado (live preview).
4. Plugins isolados em WebAssembly (wazero), com permissões declaradas.
