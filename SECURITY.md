# Política de segurança

## Versões suportadas

O projeto ainda não tem versões publicadas. Correções de segurança são feitas
na branch `main`; quando houver releases, apenas a mais recente receberá
correções.

## Como relatar uma vulnerabilidade

**Não abra uma issue pública** para relatar uma vulnerabilidade.

Use o relato privado do GitHub: na aba **Security** do repositório, clique em
**Report a vulnerability**. Se não for possível, envie um e-mail para
**engenheiroaraujo@outlook.com.br** com o assunto `[parallelepiped] segurança`.

Inclua, se puder:

- a versão ou o commit afetado e a versão do Windows;
- os passos para reproduzir — de preferência com um cofre mínimo de exemplo;
- o impacto que você observou ou espera.

### O que esperar

- Confirmação de recebimento em até **7 dias**.
- Uma avaliação inicial, com a gravidade e os próximos passos, em até
  **14 dias**.
- Aviso quando a correção for publicada, com crédito no registro da correção
  se você quiser.

Este é um projeto pessoal mantido por uma pessoa; os prazos acima são metas,
não garantias. Pedimos que a vulnerabilidade não seja divulgada antes de haver
uma correção ou de se passarem 90 dias do relato, o que vier primeiro.

## Escopo

O parallelepiped é um aplicativo desktop local: lê e grava arquivos Markdown
numa pasta escolhida pelo usuário, mantém um índice SQLite em
`%LOCALAPPDATA%\parallelepiped` e não faz requisições de rede.

Interessam especialmente:

- leitura, gravação ou exclusão de arquivos **fora** do cofre aberto (por
  exemplo, por caminhos de nota manipulados);
- execução de código ou de script a partir do conteúdo de uma nota, do nome de
  um arquivo ou de um projeto importado pela importação estrutural (🧠);
- perda ou corrupção de notas causada pelo app, inclusive pelo watcher ou pela
  reimportação;
- injeção de HTML na interface a partir do conteúdo do cofre.

Ficam fora do escopo:

- ataques que exigem que o invasor já controle a conta do usuário ou a
  máquina;
- vulnerabilidades em dependências que não afetam o parallelepiped — relate-as
  ao projeto de origem;
- o aviso do SmartScreen sobre o executável não assinado.

## Limitações conhecidas

- **Links simbólicos não são resolvidos** na checagem que confina os caminhos
  ao cofre. Abrir um cofre de origem não confiável que contenha links
  simbólicos apontando para fora dele pode fazer o app ler ou gravar nesses
  destinos.
- O executável **não é assinado**. Compile a partir do código-fonte se preferir
  não confiar no binário.
