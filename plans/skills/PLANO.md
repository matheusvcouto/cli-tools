# Plano de implementação — CLI `skills`

> **Execução:** direto na sessão, tarefa por tarefa, no checkout normal do
> cli-tools. **Sem subagentes e sem worktree.** Cada passo tem caixa (`- [ ]`).
> Antes de qualquer tarefa, ler §1 (segurança) e §2 (erros possíveis).

**Objetivo:** a CLI descrita em [`README.md`](README.md), com as regras de
[`ANALISE.md`](ANALISE.md).

**Arquitetura:** `internal/skills/core` puro (sem IO) + adaptadores; ver ANALISE §8.1.

**Tecnologia:** Go 1.27.1 (go.mod da suíte), `CGO_ENABLED=0` no build de release,
CLI Core em `cli/`, `internal/{safefs,fscommit,filelock,toolhome}`.
Dependência nova única: `go.yaml.in/yaml/v3` (Fase 1, com ADR).

**Marcadores:** **[dono]** decidido por você · **[eng]** decisão técnica ·
**[validar]** depende de sonda (Fase 0).

---

## 1. Segurança — regras inegociáveis

### 1.1 Durante a implementação (o que eu, agente, faço)

| # | Regra |
|---|---|
| S1 | **Nunca** ler para escrever, escrever, mover ou apagar em `~/.claude`, `~/.agents`, `~/.codex`, `~/.cli-tools`, `~/.ai-profiles` reais. Leitura só para diagnóstico, e só quando necessário. |
| S2 | O binário `skills` só roda, durante o desenvolvimento, com `HOME`, `CLI_TOOLS_HOME` e destinos **sintéticos** em pasta temporária. Nunca contra as pastas reais. |
| S3 | Não tocar nos repos `skills` e `private-skills`: nada é movido, renomeado ou apagado (`skill-sync` em Deno, `.output/`, `.env`, `input/`, `acp-integration-old/` continuam onde estão). Eles são **lidos** apenas por sondas, e nem isso sem necessidade. |
| S4 | Não alterar o código de `ai-profile`, `repo-zip`, `media-get` nem a API pública `cli/` (o gate `release api check` precisa continuar dizendo "no breaking drift"). Se uma tarefa parecer exigir isso, **parar e perguntar**. |
| S5 | Proibido `os.RemoveAll`, `rm -rf` ou equivalente em código de produto fora de pastas temporárias criadas pela própria operação e validadas. |
| S6 | Commits locais por tarefa, direto na `main` (padrão do repo). **Push só com seu pedido.** Nada de tag, `release prepare --write` ou release sem pedido explícito. |
| S7 | Antes de cada commit: `./scripts/check-safe.sh all` verde. Se algo não rodou, registrar "não executado", nunca "aprovado". |
| S8 | Testes nunca usam rede, contas, `~/.claude` ou `~/.agents` reais; fixtures nascem sintéticas; nomes fictícios (`alpha`, `beta`, `demo-skill`). |
| S9 | Dúvida sobre intenção, escopo ou algo destrutivo: **perguntar antes**. |

### 1.2 No produto (o que a CLI garante a você)

| # | Garantia | Como |
|---|---|---|
| P1 | **Nada é apagado de verdade.** `uninstall` e `update` movem a cópia antiga para `~/.cli-tools/skills/backups/`; `skills restore <nome>` traz de volta. | `fscommit` + mover para backups antes da troca |
| P2 | Só mexe no que **ela mesma instalou** (entrada no lock + hash conferido). Pasta desconhecida, symlink, `synced/`, arquivos soltos e instalações da Vercel nunca são tocados. | lock por destino; `Lstat` |
| P3 | **Mostra antes, aplica depois.** Todo comando que escreve imprime o plano (o quê, de onde, para onde, caminhos reais) e pede confirmação. `--dry-run` em todos eles. | `core` gera o plano; `cli` só executa o plano aprovado |
| P4 | `--yes` pula a pergunta, **não** as proteções: destino modificado à mão, pasta não gerenciada e downgrade continuam bloqueados até você confirmar item por item (ou flag específica, nunca genérica). Sem `--force`. | estados bloqueantes no `core` |
| P5 | Sem terminal (script, pipe) e sem `--yes`: nenhum comando que escreve executa. | `Interaction` não interativa = erro |
| P6 | Comandos de leitura (`list`, `status`, `diff`, `doctor`) **não escrevem nada**, nem criam `~/.cli-tools`. O `config.toml` é criado só por `skills config init` ou pela primeira operação que escreve. | adaptadores read-only |
| P7 | Cópia nunca deixa skill pela metade: copia para temporário na mesma pasta, confere hash, troca. Falha, Ctrl-C ou disco cheio → original intacto, temporário removido. | `safefs` + `fscommit` |
| P8 | Destino com raiz de agente ausente (`~/.claude` não existe) → erro explícito, nunca cria a pasta do agente. Em projeto sem `.claude/`/`.agents/`, pergunta antes de criar. | checagem antes do plano |
| P9 | Duas execuções ao mesmo tempo no mesmo destino: a segunda espera ou falha com `DEST_BUSY`; nunca intercalam. | `filelock` |
| P10 | Lock de versão mais nova: só leitura (`LOCK_TOO_NEW`). Lock corrompido: nada é escrito; mensagem diz o arquivo e o motivo. | validação + JSON Schema nos testes |
| P11 | Limpeza de backups (> 30 dias) só remove pastas **dentro** de `backups/` com o nome e o marcador que ela mesma criou; o comando espera a limpeza terminar; erro de limpeza é só aviso. | `safefs.Root` em `backups/` |
| P12 | `build` nunca sobrescreve um `.skill` diferente que já exista; bytes iguais = nada a fazer. | comparação antes de publicar |
| P13 | Nenhuma rede, nenhuma telemetria. | sem `net` no módulo `skills` (teste de arquitetura) |

---

## 2. Erros possíveis — e como cada um é evitado

### 2.1 Erros de processo (que eu posso cometer ao implementar)

| # | Erro | Prevenção |
|---|---|---|
| E1 | Rodar a CLI ou um teste contra suas pastas reais. | S1, S2; teste de integração com `testenv.New`; a CLI aceita `CLI_TOOLS_HOME` e `HOME` sintéticos. |
| E2 | Quebrar outra CLI da suíte ao mexer em pacote compartilhado. | S4; `check-safe.sh all` antes de cada commit. |
| E3 | Mudar a API pública `cli/` sem perceber (ex.: acrescentar método em `Interaction`). | Multi-select fica em `internal/skills/term`; `release api check` no gate. |
| E4 | Esquecer change record (`changes/*.json`) → CI vermelha. | Cada tarefa que toca `cmd/` ou `internal/` cria/atualiza o record (`skills` + `module`). |
| E5 | Dependência nova sem ADR, ou importada fora do pacote isolado. | ADR na Tarefa 1.4; teste de arquitetura (Tarefa 0.6). |
| E6 | Alterar module path, go.mod além do necessário ou versão do Go. | Proibido (AGENTS §7); só `go get go.yaml.in/yaml/v3@<versão exata>`. |
| E7 | Teste que depende de locale, ordem do `ReadDir`, fuso ou hora atual. | Ordenação por bytes; `Clock` falso; `LC_ALL=C` só no git. |
| E8 | Teste instável com goroutine/tempo (limpeza de backups). | Sem `sleep`; sincronização explícita; `-race` e `shuffle` no gate. |
| E9 | Comparar paths por texto (`/var` × `/private/var` no macOS). | `os.SameFile` para identidade; `filepath.Rel` para containment (AGENTS §4). |
| E10 | Interpretar stderr do git. | Só exit code + `-z`; testes com git real em repo temporário (Apple Git do CommandLineTools). |
| E11 | Supor como Claude Code/Codex leem skills sem verificar. | Sondas da Fase 0, registradas antes do código. |
| E12 | Over-engineering (repetir o `skill-sync` de 4.900 linhas). | Máx. 8 estados; sem registry/journal/plan file; cada tarefa só o que o README promete. |
| E13 | Declarar "funciona" sem rodar. | Cada tarefa termina com comando de verificação e saída esperada. |
| E14 | Commitar lixo (`.DS_Store`, binário, `dist/`). | `git add` por caminho explícito; `git status` antes do commit. |
| E15 | Bump manual de versão ou release sem pedido. | S6; versão só por `tools/release prepare` quando você pedir. |
| E16 | Esquecer Windows: código que só compila no Unix. | `GOOS=windows go vet` e build no gate; plataforma sem suporte = erro explícito. |
| E17 | Escrever texto/erros em inglês para você ou em português no código. | Mensagens ao usuário em português; identificadores em inglês (padrão do `media-get`). |
| E18 | Escopo maior que o pedido. | Fora do escopo do README fica fora; ideias vão para `ideias.md`. |

### 2.2 Erros de produto (bugs que a CLI pode ter) — cada um vira teste

| # | Bug possível | Teste / regra |
|---|---|---|
| B1 | Descoberta perde skill sem avisar ou depende da ordem (Vercel #2528). | `TestDiscoveryOrderIndependent` (fixtures em várias ordens), `doctor` lista ignoradas. |
| B2 | "Update" fantasma (Vercel #2512). | `TestStatusIdenticalAfterInstall`; hash conferido após cópia. |
| B3 | Hash diferente entre máquinas (Vercel #2448). | `TestHashGolden` com vetores fixos; ordem por bytes; moldura por arquivo. |
| B4 | Lock e disco divergem; projeto = home (Vercel #2469). | `TestProjectEqualsGlobalRejected`; lock dentro da raiz. |
| B5 | Sobrescrever edição feita à mão no destino. | `TestModifiedDestinationBlocked` (inclusive com `--yes`). |
| B6 | Apagar/seguir symlink ou pasta não gerenciada (ex.: `find-skills`, `synced/`). | `TestUnmanagedAndSymlinkNeverTouched`. |
| B7 | Skill pela metade após falha/Ctrl-C/disco cheio. | `TestCopyFailureKeepsOriginal` (falha injetada no meio). |
| B8 | Duas execuções concorrentes. | `TestConcurrentUpdateSerialized`. |
| B9 | Lock corrompido ou de versão nova reescrito. | `TestLockTooNewReadOnly`, `TestLockCorruptNoWrite`. |
| B10 | YAML: `3.10` vira `3.1`; `metadata` não string; `---js`. | `TestFrontmatterVersionMustBeString`, `TestRejectsJSFrontmatter`. |
| B11 | Nome duplicado (ex.: `acp-integration-old`). | Origem git respeita `.gitignore`; `TestDuplicateNameIsError`. |
| B12 | Injeção de escape de terminal pelo nome/descrição. | `TestSanitizeTerminalEscapes`. |
| B13 | Bit de execução perdido em `scripts/`. | `TestExecutableBitPreserved` (Unix); entra no hash. |
| B14 | Sistema de arquivos sem diferença de maiúsculas (macOS) e Unicode NFC/NFD em nomes. | Nomes de skill são `a-z0-9-` (sem ambiguidade); identidade de destino por `os.SameFile`; caminhos internos vindos do git com `-z`. |
| B15 | Arquivo gigante ou skill com milhares de arquivos. | Limites (arquivos, bytes por arquivo, total) com erro claro. |
| B16 | Terminal preso em modo raw após pânico/Ctrl-C. | `defer term.Restore`; cursor reexibido; teste do modelo de seleção separado do terminal. |
| B17 | Sem TTY, a lista trava esperando tecla. | Sem TTY → exige nomes/`--all`; fallback textual numerado como no `media-get`. |
| B18 | Limpeza de backups apaga o que não devia. | `TestPruneOnlyOwnBackups` (pasta estranha dentro de `backups/` sobrevive). |
| B19 | Goroutine de limpeza interrompida no meio ou vazando. | `TestPruneWaitsAndCancels`. |
| B20 | Git ausente ou repo inválido. | Erro `SOURCE_UNAVAILABLE` com ação; nunca cai para leitura de disco sem avisar. |
| B21 | Alterações não commitadas na origem passam despercebidas. | `status` marca "origem com alterações não commitadas" (informativo). |
| B22 | Versões não comparáveis (`2.5` × `0.1.0-beta.1` × ausente). | `TestVersionOrdering` com tabela. |
| B23 | `build` gera zip diferente a cada execução. | `TestBuildReproducible` (dois builds = mesmos bytes). |

### 2.3 Erros de decisão

| # | Risco | Prevenção |
|---|---|---|
| D1 | Destino do Codex errado (`~/.agents/skills` × `~/.codex/skills`). | Sonda 0.2 antes de fixar; até lá, só o que o README diz. |
| D2 | Comportamento que você não pediu (ex.: criar categorias, migrar repo). | Categorias opcionais **[dono]**; nada de migração no plano. |
| D3 | Divergir do README sem avisar. | README é o contrato: mudança nele vem primeiro, com sua aprovação. |

---

## 3. Roteiro

| Fase | Entrega | Aceite observável |
|---|---|---|
| 0 | Sondas + esqueleto `cmd/skills` + teste de arquitetura | Respostas das sondas no README; `skills --help` e `skills --version` funcionam; `check-safe.sh all` verde. |
| 1 | `core` (descoberta, especificação, hash, versões, estados) + `list`, `status`, `doctor` (só leitura) | Contra cópias sintéticas dos seus repos, `list` mostra as 9 skills e `doctor` aponta as 6 sem versão; nada é escrito. |
| 2 | Destino (lock, troca segura, backups, restore) + `install`, `update`, `uninstall`, `restore`, `diff` + lista com toggle | Em HOME sintético: instalar, editar à mão, ver `modificado`, atualizar com backup, restaurar; Ctrl-C no meio não deixa sujeira. |
| 3 | `build` + índice de destinos (`--all`) + `config init` / `sources` | `.skill` reprodutível; `status --all` cobre global + projetos sintéticos. |
| 4 | Uso real (com você) | Você roda `skills status --global` nas suas pastas; só depois disso, `install`. |

Só as Fases 0 e 1 têm passo a passo agora. As Fases 2 e 3 são detalhadas no
começo de cada uma, porque dependem das sondas e dos nomes reais do código.

---

## 4. Fase 0 — Sondas e esqueleto

### Tarefa 0.1 — Sonda: o Claude Code e arquivos soltos em `skills/` **[validar]**

- [ ] Criar pasta temporária `T` com `T/.claude/skills/demo-skill/SKILL.md` e `T/.claude/skills/.skills-lock.json`.
- [ ] Você roda (não eu, porque usa sua conta): `cd T && claude` e pergunta "quais skills você tem?".
- [ ] Registrar no README: a skill aparece? algum aviso sobre o arquivo solto?
- **Aceite:** resposta registrada. Se houver aviso, o lock muda de lugar (ex.: `T/.claude/.skills-lock.json`) antes da Fase 2.

### Tarefa 0.2 — Sonda: onde o Codex procura skills **[validar]**

- [ ] Ler a documentação oficial do Codex sobre skills (fonte e data registradas).
- [ ] Projeto temporário com `.agents/skills/demo-a/` e `.codex/skills/demo-b/`; você roda `codex` e pergunta as skills.
- [ ] Registrar: global = `~/.agents/skills`, `$CODEX_HOME/skills` ou ambos.
- **Aceite:** tabela "Destinos" do README atualizada com a resposta.

### Tarefa 0.3 — Sonda: upload do `.skill` no Claude.ai **[validar]**

- [ ] Gerar, com `zip -X` e data fixa, um `demo-skill-v0.1.0-beta.1.skill` sintético.
- [ ] Você faz o upload no Claude.ai.
- **Aceite:** aceito ou motivo da recusa registrado.

### Tarefa 0.4 — Sonda: skill atualizada sem reiniciar **[validar]**

- [ ] Na pasta de 0.1, alterar a descrição de `demo-skill` com o Claude Code aberto.
- **Aceite:** registrar se precisa reiniciar; o `update` passa a avisar quando precisar.

### Tarefa 0.5 — Esqueleto `cmd/skills`

**Arquivos:** criar `cmd/skills/{main.go,tool.json,README.md,ADR.md,CHANGELOG.md,cli.contract.json}`,
`internal/skills/cli/app.go`, `changes/skills-skeleton.json`.

- [ ] `tool.json`: `{"schema_version":1,"name":"skills","version":"0.1.0","stability":"experimental"}` (valor aceito por `internal/version`, o mesmo do `media-get`).
- [ ] `main.go` no padrão de `cmd/repo-zip/main.go` (manifest embutido, `SignalContext`, `RenderDiagnostic`).
- [ ] `app.go`: `core.App` com `Builtins{Help, Version, Completion, Schema}` e comandos declarados sem handler real (retornam erro "ainda não implementado" com código).
- [ ] `cmd/skills/README.md` = conteúdo do `plans/skills/README.md` sem a parte de proposta; `ADR.md` com: anti-objetivos (ANALISE §8.3), lock por destino, hash v1, cópia (não symlink), dependências.
- [ ] Gerar contrato: `go run ./cmd/skills __cli contract > cmd/skills/cli.contract.json` e teste de lock byte a byte (como as outras CLIs).
- [ ] Conferir se a release/integração descobre `cmd/*` automaticamente ou se há lista fixa a atualizar (`tools/release`, `integration/e2e_test.go`, `.github/workflows/release.yml`).
- [ ] Change record: `skills` (minor, experimental) + `module` (inclusão do produto).
- **Verificar:** `go run ./cmd/skills --help`; `./scripts/check-safe.sh all` verde.
- **Commit:** `feat(skills): esqueleto da CLI skills`.

### Tarefa 0.6 — Teste de arquitetura

**Arquivos:** criar `internal/skills/arch_test.go`.

- [ ] Teste que lê os imports (`go/parser`, `parser.ImportsOnly`) de `internal/skills/core/...` e falha se aparecer: `os`, `os/exec`, `net`, `net/http`, `io/fs` com acesso a disco via `os.DirFS`, `golang.org/x/term`, `go.yaml.in/yaml/v3`, `internal/safefs`, `internal/fscommit`, `internal/filelock`, `internal/toolhome`.
- [ ] Segunda regra: só `internal/skills/frontmatter` pode importar `go.yaml.in/yaml/v3`.
- [ ] Terceira regra: nenhum pacote de `internal/skills/...` importa `net`/`net/http` (P13).
- [ ] Fixture `testdata/bad/` com uma violação de cada regra para provar que o teste pega.
- **Verificar:** teste falha com a fixture e passa no código real.
- **Commit:** `test(skills): regra de dependência do núcleo`.

---

## 5. Fase 1 — Núcleo e comandos de leitura

### Tarefa 1.1 — Tipos do núcleo e cenários

**Arquivos:** `internal/skills/core/{model.go,scenario_test.go}`, `internal/skills/core/testdata/scenarios/*.json`.

**Interfaces (forma):**

```go
type File struct { Path string; Executable bool; Data []byte }   // Path com "/", relativo à skill
type SourceTree struct { Name string; Root string; Files []File; Dirty bool }
type Skill struct { Name, Description, Version, Category, Source string; Internal bool; Files []File; Hash string }
type Finding struct { Code string; Severity Severity; Skill, Path, Message string; Action string }
type State string  // novo, atualizado, update, conteudo-mudou, modificado, downgrade, nao-gerenciado, orfao
type Clock interface { Now() time.Time }
```

- [ ] Formato de cenário em JSON: origem (árvore de arquivos), destino (árvore + lock), entrada (comando/seleção) → saída esperada (estados, operações, findings).
- [ ] Teste-runner de cenários (falha primeiro: nenhum cenário passa).
- **Commit:** `feat(skills): tipos do núcleo e formato de cenários`.

### Tarefa 1.2 — Hash `skills-hash-v1`

**Arquivos:** `internal/skills/core/hash.go`, `hash_test.go`.

- [ ] Testes primeiro: vetores fixos (golden) para: arquivo único; dois arquivos em ordem invertida na entrada; `a`+`bc` × `ab`+`c`; bit de execução; nomes com acento (bytes UTF-8); CRLF preservado.
- [ ] Implementar conforme ANALISE §3.
- **Commit:** `feat(skills): hash portável de skill`.

### Tarefa 1.3 — Versões

**Arquivos:** `internal/skills/core/version.go`, `version_test.go`.

- [ ] Tabela de testes: `2.5 < 2.10`, `2.5 = 2.5.0`, `0.1.0-beta.1 < 0.1.0`, `beta.2 < beta.10`, ausente, inválida (`v1`, `1.0.0.0`) = "desconhecida" (não ordena, não pré-marca).
- **Commit:** `feat(skills): comparação de versões`.

### Tarefa 1.4 — Frontmatter (dependência YAML isolada)

**Arquivos:** `internal/skills/frontmatter/{frontmatter.go,frontmatter_test.go,fuzz_test.go}`,
`go.mod`/`go.sum`, ADR em `cmd/skills/ADR.md`, change record.

- [ ] ADR: por que `go.yaml.in/yaml/v3` (versão exata), licença, transitivas, critérios de troca (ANALISE §8.2).
- [ ] `go get go.yaml.in/yaml/v3@<versão exata>`; `./scripts/check-safe.sh prepare`.
- [ ] Testes primeiro: BOM; CRLF; sem frontmatter; `---js` recusado; `metadata.version: 3.10` (número) = erro; `metadata` com valor não string = erro; descrição multilinha (`>`/`|`); campos desconhecidos preservados como aviso, não erro.
- [ ] Fuzz do parser (padrão da suíte).
- **Commit:** `feat(skills): leitura do frontmatter`.

### Tarefa 1.5 — Validação da especificação

**Arquivos:** `internal/skills/core/spec.go`, `spec_test.go`.

- [ ] Regras da ANALISE §6 (nome, descrição, compatibility, metadata, allowed-tools, 500 linhas), cada uma com teste de entrada inválida; trim antes de validar (V7).
- [ ] Sanitização de escapes de terminal para todo texto vindo de skill (V12).
- **Commit:** `feat(skills): validação Agent Skills`.

### Tarefa 1.6 — Descoberta

**Arquivos:** `internal/skills/core/discover.go`, `discover_test.go`.

- [ ] Testes primeiro: layout na raiz (`<nome>/SKILL.md`, o atual); `skills/<nome>`; `skills/<cat>/<nome>`; `skills/<cat>/<sub>/<nome>`; skill com `examples/x/SKILL.md` dentro (não vira skill); ordem de entrada embaralhada = mesmo resultado (B1); nome duplicado = erro com os dois caminhos (B11); `metadata.internal` oculto; limites (B15).
- [ ] Toda pasta ignorada vira `Finding` (para o `doctor`).
- **Commit:** `feat(skills): descoberta determinística`.

### Tarefa 1.7 — Origem git e origem de disco (somente leitura)

**Arquivos:** `internal/skills/source/{git.go,fs.go,source_test.go}`.

- [ ] Git: `git -C <repo> ls-files -co --exclude-standard -z`, `LC_ALL=C`, argv em lista, só exit code; `git status --porcelain=v1 -z` para `Dirty` (B21). Arquivo listado que é symlink = finding de erro.
- [ ] Disco (fora de git): ignora `.git`, `node_modules`, `.DS_Store`, `.env*`.
- [ ] Testes com repo git temporário (padrão de `repo-zip`), incluindo pasta `*-old/` ignorada (reproduz R1).
- [ ] Git ausente → `SOURCE_UNAVAILABLE` (B20).
- **Commit:** `feat(skills): leitura de origens`.

### Tarefa 1.8 — Estados (comparação origem × destino), sem escrever

**Arquivos:** `internal/skills/core/status.go`, cenários; `internal/skills/dest/read.go` (leitura do lock e do destino, `Lstat`, nunca segue symlink).

- [ ] Cenários para os 8 estados, mais: symlink no destino (B6), `synced/` ignorado, lock de versão maior (B9), lock corrompido, projeto = global (B4).
- **Commit:** `feat(skills): estados de instalação`.

### Tarefa 1.9 — Comandos `list`, `status`, `doctor`

**Arquivos:** `internal/skills/cli/{list.go,status.go,doctor.go}`, `cli.contract.json`, change record.

- [ ] `--json` com JSON Schema validado nos testes; `status --exit-code`.
- [ ] Teste de integração com `testenv` + HOME sintético + cópias sintéticas que imitam seus repos (3 skills na raiz + `*-old` ignorada + 6 sem versão).
- [ ] Teste que prova P6: depois de `list/status/doctor`, nenhum arquivo novo em HOME sintético.
- **Verificar:** saída de exemplo colada no README.
- **Commit:** `feat(skills): list, status e doctor`.

**Aceite da Fase 1:** os testes acima verdes; `check-safe.sh all` verde; você
vê a saída de exemplo e aprova antes da Fase 2.

---

## 6. Fase 2 — Escrita segura (detalhar no início da fase)

1. `dest`: lock por destino (filelock + troca atômica + fsync da pasta); migração de schema; `LOCK_TOO_NEW`.
2. Cópia: temporário na mesma pasta, conferência de hash, troca com `fscommit`, versão antiga movida para `backups/` (P1, P7).
3. `install`, `update`, `uninstall` (move para backup), `restore`, `diff`; plano mostrado + confirmação + `--dry-run` (P3–P5).
4. Lista com toggle em `internal/skills/term` (modelo de seleção puro, testado sem terminal; renderização inspirada em `internal/mediaget/cli/terminal.go`, sem alterá-lo); fallback textual sem TTY.
5. Limpeza de backups > 30 dias em goroutine, com `Clock` falso nos testes (P11, B18, B19).
6. Foco de revisão: B5, B6, B7, B8, B16, B17.

## 7. Fase 3 — Build, índice e config (detalhar no início da fase)

1. `build`: zip reprodutível, nunca sobrescreve `.skill` diferente (P12, B23).
2. Índice de destinos em `~/.cli-tools/skills/state/targets.json`; `status --all` / `update --all`; caminhos que sumiram são ignorados.
3. `config init`, `sources add|list` com `toolhome.EnsureConfig`.

## 8. Fase 4 — Uso real, junto com você

1. Você roda `skills status --global` e `skills doctor` nas pastas reais (só leitura).
2. Revisamos juntos o que aparece como `não gerenciado`.
3. Primeiro `install` real de **uma** skill, com `--dry-run` antes.

---

## 9. Foco de revisão (todas as fases)

1. Nada fora do lock é tocado (B6, P2).
2. Falha no meio nunca deixa skill pela metade (B7, P7).
3. Edição manual nunca é sobrescrita sem confirmação explícita (B5, P4).
4. Hash idêntico entre execuções e máquinas (B2, B3).
5. Comandos de leitura não escrevem (P6).
6. Limpeza só remove backups próprios (B18).

## 10. Cobertura dos seus casos de uso

| Caso | Comando | Fase |
|---|---|---|
| Ver o que está desatualizado no global | `skills status --global` | 1 |
| Validar skills antes de publicar | `skills doctor` | 1 |
| Instalar global ou num projeto, escolhendo na lista | `skills install --global` / `--project <path>` | 2 |
| Atualizar só o que mudou | `skills update --global` | 2 |
| Desfazer uma atualização | `skills restore <nome>` | 2 |
| Gerar `.skill` para o Claude.ai | `skills build` | 3 |
| Repo público + privado juntos | `config.toml` com duas origens | 3 |
| Atualizar todos os projetos de uma vez | `skills update --all` | 3 |
| Organizar por categorias (quando quiser) | layout `skills/<cat>/<nome>/` | já suportado na 1 |
