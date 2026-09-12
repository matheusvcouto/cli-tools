# Status de verificação e riscos conhecidos

Complemento do `adr.md` (decisões) e do `agy-keychain-issue.md` (o bug do
keychain do agy). Aqui ficam: (1) se o isolamento realmente funciona por
CLI, com a evidência, e (2) outros riscos do desenho atual que valem
atenção, mesmo que ainda não tenham causado problema.

## Isolamento funciona? (verificado, não assumido)

### claude — ✅ funcional (confirmado empiricamente)
Isola via `CLAUDE_CONFIG_DIR`, mantendo o `$HOME` real. A credencial fica
no Keychain do macOS indexada por hash do caminho do config dir → cada
perfil tem entrada de keychain separada. Confirmado no uso real: o plano
uma segunda conta funcionou sob seu próprio perfil, separada da conta
principal.

### codex — ✅ funcional (verificado nesta investigação)
Isola via `CODEX_HOME`, mantendo o `$HOME` real. O arquivo
`<CODEX_HOME>/auth.json` contém os tokens de verdade (`id_token`,
`access_token`, `refresh_token`, `account_id`) — confirmado inspecionando
a estrutura do arquivo. Como cada perfil tem seu próprio `CODEX_HOME`,
cada um tem seu próprio `auth.json`. Existe também um item "Codex Safe
Storage" no Keychain (padrão estilo Chrome), mas como o `$HOME` real é
mantido, o keychain é acessível normalmente; e o token real está no
arquivo, então o isolamento não depende dele.

### agy — ❌ removido do módulo (decisão final, ver `agy-keychain-issue.md`)
A credencial real do agy é um item ÚNICO e GLOBAL do Keychain
(`gemini`/`antigravity`), não por config dir. Isolar via `$HOME` (única
opção, já que o agy não tem env var dedicada) quebrava o keychain padrão
(dialog "Chaves Não Encontradas") e não dava isolamento de verdade — só
funcionava por acidente, via fallback de arquivo de token em texto puro.
Multi-conta robusta exigiria um Keychain por perfil (Tipo C, frágil,
macOS-only, não implementado). Decisão: tirar o `agy` do `TOOLS` e usá-lo
direto, sem perfil — prioriza zero risco de keychain sobre multi-conta.

## Outros riscos do desenho atual

### 1. `index.nuon` é ponto único de falha, sem recuperação — ALTO
Desde que o fallback "legado" foi removido (ver `adr.md`), `profile-list`
e `profile-path` dependem 100% de `~/.ai-profiles/index.nuon`. Se esse
arquivo for apagado, esvaziado ou corrompido:

- Todos os perfis somem da listagem, **mesmo com as pastas intactas no
  disco**.
- As pastas viram órfãs: têm nomes opacos (ex: `agy-20260621173337-omro`)
  e **não há comando no módulo pra listar/re-adotar** essas pastas. A
  única recuperação é editar o `index.nuon` na mão.

Isso **já aconteceu uma vez** nesta sessão (o índice apareceu como `[]`
depois de uma exclusão). Naquele caso foi intencional, mas mostra que o
arquivo pode ficar vazio e que não há rede de segurança.

Mitigações possíveis (não implementadas):
- Escrever o índice de forma atômica (arquivo temp + rename) pra evitar
  corrupção em escrita interrompida.
- Manter um `.bak` do índice (como o próprio codex faz com
  `.codex-global-state.json.bak`).
- Um comando `ai-profile <tool> adopt <dir>` ou um "doctor" que varre
  `~/.ai-profiles/` e re-registra pastas órfãs no índice.

### 2. `$HOME` override do agy quebra mais que o keychain — RESOLVIDO
Era o risco do agy isolado via `$HOME` falso: não enxergar `~/.gitconfig`,
`~/.ssh/`, rc do shell, caches duplicados (`ms-playwright-go`), etc. Não
se aplica mais — o `agy` foi removido do `TOOLS` (ver seção acima e
`agy-keychain-issue.md`). Mantido aqui só como histórico.

### 3. Sem proteção de concorrência no `index.nuon` — BAIXO
`create`/`rename`/`delete` fazem read-modify-write do índice sem lock. Dois
comandos rodando ao mesmo tempo poderiam sobrescrever a alteração um do
outro. Risco baixo no uso pessoal interativo, mas existe.

### 4. UX: `ai-profile list` (sem tool) confunde — BAIXO
Como `tool` é sempre o primeiro argumento, `ai-profile list` interpreta
`list` como nome de CLI e dá "CLI desconhecida: list". Esperado pelo
desenho, mas o erro não orienta o usuário a digitar `ai-profile <tool>
list`. Poderia ter uma mensagem mais amigável, ou um comando separado pra
listar tudo de todas as CLIs.

## Nenhum problema encontrado em

- Lógica de `rename` (só edita alias, preserva dir/created_at) — testada.
- `safe-remove` (recusa apagar fora do root) — testada.
- Passagem de flags no `run` sob `--wrapped` — testada.
- Validação de tool desconhecida no topo do dispatcher — testada.
