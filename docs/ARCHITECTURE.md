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
    watcher/               observa o cofre (fsnotify) e reindexa mudanças feitas fora do app
    brain/                 importação estrutural: mapeia o código do cofre em notas (docs/cerebro)
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
    vaultpath/             regra única de pastas ignoradas (ocultas, vendor, node_modules)
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
  Na composition root quem o implementa é o `watcher/service.Service`, que repassa ao
  índice e registra o conteúdo gravado pelo próprio app.
- `vault/service.Watcher` — ao abrir um cofre, o vault para o watcher anterior antes de
  reconstruir o índice e começa a observar a nova pasta depois.
- `watcher/service.Indexer` — o watcher atualiza o índice (`Upsert`/`Remove`/`Rebuild`).
- `brain/service.Notes` — a importação lê, grava e apaga notas com strings simples; o
  adaptador `brainNotes` no `main.go` repassa ao serviço de notas, para que o watcher
  reconheça essas escritas como do próprio app.

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

### Layout e temas do frontend

Layout "Pedreira" (protótipos em `design/layouts/`): trilho de ícones, explorador
com textura de granito, abas e editor em "folha", e coluna de contexto da nota
(mini-mapa 2D da vizinhança, backlinks, ligações de saída e tags — as duas
últimas lidas do texto do editor, sem depender do índice).

As cores de pasta saem de um só lugar (`frontend/src/ui/colors.ts`), então a
árvore, as abas, o contexto e o grafo concordam. Os temas escuro e claro são
tokens CSS em `html[data-theme]` (`frontend/src/ui/theme.ts`); a escolha fica no
`localStorage` e, até o usuário escolher, segue o sistema. O grafo 3D troca
fundo, névoa e rótulos com o tema e desliga o bloom no tema claro.

### Desempenho do grafo 3D

O grafo renderiza sob demanda: desenha enquanto o layout se acomoda, a câmera
anima ou há interação (ponteiro, roda), e pausa após 1,5 s de inatividade; fora
da aba do grafo fica pausado. Os pulsos só correm nas fibras do neurônio sob o
mouse. A resolução é a nativa da tela (até 2×), sem supersampling.

### O arquivo é a fonte da verdade

As notas são arquivos Markdown comuns. O índice SQLite (em
`%LOCALAPPDATA%\parallelepiped\index\`) é cache: se uma atualização do índice
falhar depois que o arquivo foi gravado, a falha é registrada no log e a
gravação não é desfeita. Abrir o cofre de novo reconstrói o índice.

### Watcher do cofre

`watcher/repository.FSSource` coloca um watch do fsnotify em cada pasta visível
do cofre (o fsnotify não é recursivo; pastas ocultas como `.git` e `.trash` ficam
de fora) e acrescenta pastas novas conforme aparecem. Os eventos vão para
`watcher/service`, que espera cada caminho ficar 300 ms quieto
(`model.Debouncer`) e então lê o arquivo do disco:

- existe → `Upsert`; não existe mais → `Remove`;
- pasta removida/renomeada, ou eventos perdidos (overflow) → `Rebuild` do cofre,
  já que o índice não sabe listar as notas de uma pasta.

**Escritas do próprio app.** O `FileRepository` grava num `.write-*.tmp` (oculto,
ignorado) e renomeia sobre a nota, o que gera eventos na nota. Como as notas
escrevem no índice através do watcher, ele guarda o hash do conteúdo indexado
(`model.Ledger`); se o disco tem exatamente esse conteúdo — ou se a nota já foi
removida pelo app — o evento é descartado sem reindexar nem notificar. Não há
janela de tempo: uma edição externa logo depois de um save é reindexada.

Cada lote sincronizado gera o evento Wails `vault:changed` com
`{"paths": [...]}` (caminhos relativos; uma pasta cobre tudo abaixo dela e `""`
é o cofre inteiro). `watcher/handler.Notifier` emite via `platform/desktop`; não
é ligado ao `Bind`, porque o frontend só escuta. O frontend atualiza a árvore,
os backlinks da nota aberta e o grafo visível, e recarrega a nota aberta se ela
mudou e não há edição pendente (se sumiu, fecha o editor com um aviso).

O watcher é encerrado no `OnShutdown` do Wails, antes de o índice ser fechado.

### Importação estrutural do projeto (🧠)

Sem IA e sem rede: `brain/repository.Analyzer` percorre o cofre como um projeto de
código (ignora `vendor`, `node_modules`, ocultas, `dist`, `build`, `bin`,
`testdata`, `wailsjs` e o próprio `docs/cerebro`). Cada pasta com código vira uma
unidade; em Go lê pacote, doc, símbolos exportados e imports internos ao módulo
(`go/parser`); em TS/JS resolve imports relativos.

`brain/model` gera, de forma determinística, uma nota-índice
(`docs/cerebro/<projeto>.md`), uma nota-hub por módulo (`docs/cerebro/<módulo>/Módulo
<módulo>.md`, o módulo é a primeira pasta que não é invólucro como `internal`,
`modules`, `pkg`, `src`) e uma nota por unidade, com `[[links]]` de dependência nos
dois sentidos. O título da unidade é o menor sufixo único do caminho
(`vault.service`), sem colidir com notas existentes.

Regras de segurança na reimportação:

- notas geradas levam `gerado-por: parallelepiped` no front matter; uma nota sem
  essa marca **nunca** é sobrescrita (é reportada como ignorada);
- a seção `## Minhas anotações` é do usuário e sobrevive a toda reimportação;
- conteúdo idêntico não é regravado; notas geradas cuja pasta sumiu são
  removidas (para `.trash/`), exceto se tiverem texto do usuário.

O progresso vai ao frontend pelo evento `brain:progress` (`{done, total}`).

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

1. Renomear nota reescrevendo os `[[links]]` que apontam para ela.
2. Pré-visualização do Markdown renderizado (live preview).
3. Plugins isolados em WebAssembly (wazero), com permissões declaradas.
4. Watcher: avisar quando a nota aberta muda fora do app enquanto há edição
   pendente (hoje a versão do editor vence no próximo save) e reindexar só a
   pasta afetada em vez de reconstruir o cofre inteiro.
