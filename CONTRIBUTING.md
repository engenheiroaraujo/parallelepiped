# Contribuindo com o parallelepiped

Obrigado pelo interesse! Este guia explica como preparar o ambiente, como o
código está organizado e o que se espera de uma contribuição.

Ao participar, você concorda em seguir o [Código de Conduta](CODE_OF_CONDUCT.md).
Vulnerabilidades **não** devem ser abertas como issue pública — veja
[SECURITY.md](SECURITY.md).

## Antes de começar

- **Bugs:** abra uma issue com os passos para reproduzir, o que você esperava,
  o que aconteceu e a versão do Windows. Se possível, anexe um cofre mínimo
  (algumas notas `.md`) que mostre o problema — nunca notas pessoais.
- **Ideias e mudanças grandes:** abra uma issue para conversar **antes** de
  escrever o código. Isso vale principalmente para mudanças de arquitetura,
  novas dependências e qualquer alteração visual.
- **Correções pequenas** (texto, documentação, bug óbvio) podem ir direto para
  um pull request.

## Ambiente

Requisitos: Go 1.26+, Node.js 22+ com npm, WebView2 (já vem no Windows 10/11)
e, opcionalmente, GNU make.

```bash
make tools   # instala o Wails CLI v2.16.0
make deps    # baixa módulos Go e pacotes do frontend
make dev     # abre o app com hot reload
```

Para testar, use um cofre de exemplo — qualquer pasta com alguns arquivos `.md`
serve. O app só lê e grava dentro da pasta aberta, e notas apagadas vão para
`.trash/` dentro dela.

## Como o código está organizado

O backend é um monólito modular em camadas, descrito em
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Leia esse documento antes de
mudar algo no Go. Em resumo:

- Cada módulo em `internal/modules/<nome>` tem `handler`, `service`, `model`,
  `repository` e `dto`, com dependência numa só direção:
  `handler → service → repository`.
- Módulos conversam por interfaces pequenas definidas por quem consome, nunca
  importando o handler ou o model de outro módulo.
- Infraestrutura transversal fica em `internal/platform`. Erros sentinela
  vivem no catálogo `internal/platform/errors`.
- O schema do índice fica em `migrations/` na raiz. Uma mudança de schema é
  **uma migration nova** (`<versão>_<descrição>.sql`); migrations já
  publicadas não são editadas.
- Os arquivos `.md` são a fonte da verdade. O índice SQLite é cache e precisa
  continuar reconstruível a partir deles.
- O frontend fica em `frontend/src`: views em `views/`, cliente da API em
  `api/`, cores e tema compartilhados em `ui/`. Os bindings em
  `frontend/wailsjs` são gerados — rode `make bindings` em vez de editá-los.

### Idioma

- Código, identificadores, comentários, mensagens de erro internas, logs e
  mensagens de commit: **inglês**.
- Textos da interface e mensagens mostradas ao usuário: **português**.
- Documentação do repositório: português.

## Mudanças visuais e de desempenho

O visual do app — em especial o grafo 3D de neurônios — é parte do produto.

- Uma mudança **visual** precisa de capturas de tela de antes e depois, nos
  temas claro e escuro, no pull request.
- Uma mudança de **desempenho** não pode alterar o resultado visual. Descreva
  como mediu (antes e depois) e como verificou que a imagem continua igual.

## Pull requests

1. Crie um branch a partir da `main` atualizada.
2. Mantenha o PR focado em um assunto; refatorações não relacionadas vão em
   outro PR.
3. Adicione ou ajuste testes para o comportamento alterado. Regras puras
   ficam testadas no `model`; casos de uso, no `service` com fakes.
4. Rode a verificação completa antes de enviar:

   ```bash
   make fmt
   make check   # go vet, testes Go e checagem de tipos do frontend
   ```

5. Atualize a documentação (`README.md`, `docs/ARCHITECTURE.md`) quando o
   comportamento ou a arquitetura mudarem.
6. Escreva commits no imperativo, em inglês, explicando o porquê quando não
   for óbvio — por exemplo, `Restyle only the hovered neuron's links`.

Não inclua no PR arquivos gerados pelo build (`build/bin`, `frontend/dist`),
cofres pessoais nem dados reais de notas.

## Licença

O projeto é distribuído sob a [licença MIT](LICENSE). Ao enviar uma
contribuição, você concorda que ela seja licenciada sob os mesmos termos.
