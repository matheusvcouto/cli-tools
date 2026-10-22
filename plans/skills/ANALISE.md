# Análise crítica — o que aprender antes de planejar

Fontes, em 21/10/2026:

- `vercel-labs/skills` v1.7.4 (código-fonte, commit `a80e4c9`) e as issues abertas;
- especificação Agent Skills (agentskills.io/specification);
- o `skill-sync` em Deno e as skills dos repos `skills` e `private-skills`;
- o estado real de `~/.claude/skills` e `~/.agents` nesta máquina (só leitura).

Cada item diz **o que aconteceu**, **a lição** e **a regra para o `skills`**.

## 1. Erros da Vercel que não vamos repetir

| # | O que aconteceu | Regra para o `skills` |
|---|---|---|
| V1 | **#2528** — o layout com 2 níveis de categoria perde skills sem avisar, e o resultado depende da ordem das pastas (11 de 14, depois 1 de 14). | Descoberta determinística (ordem pelos bytes do caminho). Toda pasta ignorada ou escondida aparece no `doctor` com o motivo. `list` mostra o total. Teste com fixture de categorias em ordens diferentes. |
| V2 | **#2512** — "update" fantasma: a mesma skill "atualiza" em toda execução porque compara o estado do repo, não o da skill. | Comparação **por skill**, pelo hash do conteúdo. Hash igual = nada a fazer. Depois de instalar, recalcula o hash do destino e confere com a origem. |
| V3 | **#2448** — hash não portável: ordenação dependente do locale, `a`+`bc` = `ab`+`c`, symlinks fora do hash mas copiados, CRLF. | Algoritmo especificado e versionado (§3). Ordem por bytes UTF-8, cada arquivo com moldura, o hash cobre **exatamente** o conjunto que é copiado. |
| V4 | **#2469** — `remove` com cwd = home apaga a skill global e deixa o lock sujo: o escopo do disco e o do lock divergem. | O lock mora **dentro** da raiz que ele descreve; não há como divergir. Se `--project` resolver para a mesma pasta física que o global (`os.SameFile`), é erro explícito. |
| V5 | Lock com versão antiga é **apagado** ("wipe"): o usuário perde o rastreio e precisa reinstalar tudo. | `schema_version` com migração. Versão desconhecida (mais nova) = recusa com mensagem, sem tocar. Nunca apaga. |
| V6 | **#2504** — `--skill` com match exato falha para variações de nome sem avisar. | Os nomes já são canônicos pela especificação. Seletor = nome exato ou prefixo de categoria (`marketing/`). Seletor sem resultado é erro com sugestões, nunca é ignorado. |
| V7 | **#2472** — metadado só com espaços é aceito. | Valida depois do trim. |
| V8 | **#2503** — `init` grava YAML sem aspas e gera SKILL.md quebrado. | O `skills` **não grava** `SKILL.md`. Só lê. |
| V9 | **#2505** — detecção de erro do git por texto em inglês quebra com o git traduzido. | Nunca interpretar stderr do git. Usar só exit code e saída `-z`. Rodar o git com `LC_ALL=C`. |
| V10 | **#2480** — com muitas skills a lista fica enorme; pedem "grupos primeiro". | Lista agrupada por categoria, filtro com `/` e marcar/desmarcar a categoria inteira. Seleção por categoria na linha de comando (`marketing/`). |
| V11 | **#2524** — não há como atualizar todos os projetos onde já se instalou algo. | Índice leve dos destinos já usados para `status --all`/`update --all` (decidido: automático, em `~/.cli-tools/skills/state/`). |
| V12 | Nome e descrição vindos do SKILL.md podem trazer sequências de escape de terminal (CWE-150); a Vercel tem `sanitize.ts`. | Remover escapes e caracteres de controle de todo texto vindo de skill antes de imprimir. |
| V13 | Telemetria ligada por padrão; já houve vazamento de nomes de repos privados para o índice público (#2500). | Zero rede, zero telemetria. A CLI não precisa de internet. |
| V14 | Módulo inteiro (`skill-relocation.ts`) para skills que mudam de pasta. | Lock indexado pelo **nome**, não pelo caminho. Mudar de categoria não muda nada no destino; o `status` mostra a categoria nova. |

## 2. O que a Vercel faz bem e vamos copiar

- Layout `skills/<cat>/<nome>/SKILL.md` com profundidade limitada; o nível mais
  raso esconde o que está abaixo (exemplos e testes dentro de uma skill não viram skills).
- `metadata.internal: true` esconde skills em andamento.
- Separar "olhar" (`check`) de "aplicar" (`update`).
- Hash que inclui o caminho relativo (detecta renomeação).
- Frontmatter só YAML: sem `---js` (que permitiria executar código), BOM e CRLF tolerados.
- Limites de tamanho e de quantidade de arquivos.
- Atualizar o binário por meio do instalador (mise), não por conta própria.

## 3. Hash e identidade (contrato)

`skills-hash-v1`, SHA-256 sobre:

```text
para cada arquivo, em ordem crescente dos bytes UTF-8 do caminho relativo (separador "/"):
  caminho \0 modo \0 tamanho \0 conteúdo
modo = "x" se executável, "-" se não
```

- Conteúdo bruto (sem normalizar CRLF): o que se copia é o que se hasheia.
- O bit de execução entra no hash porque `scripts/` precisa dele.
- Symlink dentro da skill: **recusado** na v1 (nenhuma skill atual usa).
- Arquivos excluídos (§4) não entram no hash nem na cópia: um único conjunto para as duas coisas.
- O lock grava `"hash": "skills-hash-v1:sha256:…"`. Se o algoritmo mudar, o prefixo muda.

## 4. Achados nos seus repos

| # | Achado | Consequência |
|---|---|---|
| R1 | `acp-integration-old/` declara `name: acp-integration`, igual à atual. Só fica fora do git por causa do `*-old/` no `.gitignore`. | Origem que é repo git usa a **visão do git**: arquivos rastreados + novos não ignorados (`git ls-files -co --exclude-standard -z`). O `.gitignore` vale para descoberta, hash, install e build. Fora do git: filtro padrão (`.git`, `node_modules`, `.DS_Store`, `.env*`). Nome duplicado = erro com os dois caminhos. |
| R2 | Versões no formato `"3.1.1"`, `"2.5"` e `"0.1.0-beta.1"`; nenhuma das 6 skills do `private-skills` tem versão. | Comparação aceita `X.Y`, `X.Y.Z` e pré-release SemVer (`beta.1 < 1.0.0`). Sem versão = só hash, com aviso no `doctor`. Versão não reconhecida = "versão desconhecida": a lista mostra, não pré-marca. |
| R3 | YAML: `version: 3.10` sem aspas vira o número `3.1`. A especificação exige `metadata` com valores string. | `doctor` dá erro se `metadata.version` não for string. Lê com parser YAML real. |
| R4 | Cada skill tem `CHANGELOG.md`. | Ideia futura: mostrar o trecho do changelog entre a versão instalada e a nova. |
| R5 | O `.skill` atual (`.output/`) é um zip com a pasta da skill na raiz. | Manter o formato. Build **reprodutível**: entradas em ordem, data fixa, permissões normalizadas; mesmo conteúdo gera os mesmos bytes. |
| R6 | `~/.claude/skills/synced/` é do app Claude (pastas UUID, `.bucket-*`). `find-skills` é symlink para `~/.agents/skills/find-skills`, instalado pela Vercel (`~/.agents/.skill-lock.json`). | Só mexe no que está no **próprio** lock. Symlinks e pastas desconhecidas = "não gerenciado", nunca seguidos nem apagados. Nomes reservados (`synced`) nunca são instalados. |
| R7 | O global do Codex (`~/.agents/skills`) é também a "pasta canônica" da Vercel. | O lock fica em `~/.agents/skills/.skills-lock.json`, que não colide com o `~/.agents/.skill-lock.json` da Vercel. |

## 5. Lições do `skill-sync` em Deno (~4.900 linhas)

- **Funcionou como ideia:** inventário por arquivo com bit de execução; hash
  gerenciado para detectar edição no destino; limites de arquivos e bytes.
- **Excesso:** 14 estados, "grants" por tipo de conflito, registry, journal,
  arquivos de plano, run-ids, rollback multinível. A maior parte protege contra
  cenários que um lock por destino + troca segura + 1 backup já resolvem.
- **Regra:** no máximo 8 estados visíveis; um único lock por destino; nada que
  precise de "recover".

## 6. Especificação Agent Skills (o que o `doctor` valida)

| Campo | Regra |
|---|---|
| `name` | 1–64 caracteres, `a-z0-9-`, sem hífen no início/fim, sem `--`, igual ao nome da pasta. |
| `description` | 1–1024 caracteres, não vazia depois do trim. |
| `compatibility` | 1–500 caracteres, se existir. |
| `metadata` | Mapa string → string. |
| `allowed-tools` | Aviso informativo se pré-aprovar `Bash` sem escopo. |
| `SKILL.md` | Aviso acima de 500 linhas. |

Erros bloqueiam `install`/`build`; avisos não.

## 7. Engenharia dentro do cli-tools

| Tema | Situação | Encaminhamento |
|---|---|---|
| Escrita segura | Já existem `internal/safefs` (raiz confinada, recusa symlink), `internal/fscommit` (troca sem apagar antes) e `internal/filelock`. | Reutilizar. Nada de `os.RemoveAll` + copy. |
| Concorrência | Dois `skills update` ao mesmo tempo no mesmo destino. | `filelock` no lock do destino durante a transação. |
| Multi-select | O CLI Core só tem `Confirm`/`Text`/`Secret`. | Decidido: lista própria sobre `golang.org/x/term`, já no go.mod, sem trazer bubbletea. |
| YAML | Nenhuma dependência YAML na suíte. | Decidido: `go.yaml.in/yaml/v3` (parser real; YAML caseiro é fonte de bug). ADR na implementação. |
| Zip | `archive/zip` da stdlib. | Sem dependência. |
| Testes | Regras da suíte: HOME sintético, nada de `~/.claude` real. | Fixtures sintéticas de repos e destinos. |
| Saída | `--json` para scripts. | `status --exit-code` retorna 1 se houver algo a atualizar (como `git diff --exit-code`). |

## 8. Método do `agent-protocol` aplicado ao `skills`

Fonte: `agent-protocol` (ADR.md, spec/SPEC.md §1–5, §11, §16.1, §18, plans/).
Cada item diz o que o `agent-protocol` faz e o que vai para o plano do `skills`.

### 8.1 Lógica separada da implementação, com teste que garante isso

**Lá:** arquitetura hexagonal (ADR-005); `internal/core` só conhece contratos, e um
teste (`internal/archtest`, imports lidos com `go/parser`) **falha** se o núcleo
importar IO.

**No `skills`:**

```text
internal/skills/
├── core/         # puro: descoberta sobre uma lista de arquivos, validação da
│                 # especificação, hash, comparação/estados, plano de operações
├── ports.go      # contratos: Source, Destination, Selector, Clock
├── source/       # git (ls-files -z) e disco
├── dest/         # safefs + fscommit + filelock; lock e backups
├── frontmatter/  # ÚNICO pacote que importa a lib YAML
├── term/         # lista com toggle sobre x/term
└── cli/          # composição da CLI (CLI Core)
```

- `core` não importa `os`, `os/exec`, `io/fs` de disco, `safefs`, `fscommit`,
  `filelock`, `x/term` nem a lib YAML. Recebe dados (lista de arquivos com
  caminho, modo e bytes) e devolve decisões (estados e operações).
- Teste de arquitetura no próprio pacote, no estilo do `archtest`. Se funcionar
  bem, vira ideia para a suíte inteira (`ideias.md`).
- Ganho prático: quase todo comportamento (V1–V14, R1–R7) é testado em `core`
  com dados em memória, sem disco, rápido e determinístico.

### 8.2 Dependência externa isolada num pacote só

**Lá:** ADR-020 — só `internal/acp/wire` importa a biblioteca ACP; um teste
garante; o ADR lista **quando trocar** (bug sem correção, parou de ser mantida…).

**No `skills`:** só `internal/skills/frontmatter` importa `go.yaml.in/yaml/v3`
(teste garante). O ADR da CLI registra os critérios de troca. Trocar a lib vira
uma tarefa local.

### 8.3 Anti-objetivos: o que é fracasso mesmo funcionando

**Lá:** SPEC §2 separa objetivos, não-objetivos e **anti-objetivos**.

**No `skills`** (entram no ADR da CLI e cada um vira teste):

- apagar ou sobrescrever algo que não foi o `skills` que instalou;
- uma skill sumir sem aviso na descoberta;
- dizer "atualizado" quando nada mudou (ou o contrário);
- precisar de rede ou enviar qualquer dado;
- erro genérico sem dizer o que fazer;
- teste que toca `~/.claude`, `~/.agents` ou `~/.cli-tools` reais.

### 8.4 Erros tipados com ação sugerida

**Lá:** SPEC §11 — `{code, message, action{kind, command}}`, igual em todas as saídas.

**No `skills`:** mesmo formato no texto e no `--json`, sobre o `Diagnostic` do CLI Core.

| Código | Quando | Ação sugerida |
|---|---|---|
| `SKILL_INVALID` | frontmatter fora da especificação | `skills doctor <nome>` |
| `NAME_CONFLICT` | mesmo `name` em dois lugares | mostra os dois caminhos |
| `SELECTOR_NOT_FOUND` | nome/categoria inexistente | sugestões parecidas |
| `DEST_MODIFIED` | destino editado à mão | `skills diff <nome>` ou confirmar na lista |
| `DEST_UNMANAGED` | pasta não instalada pelo `skills` | mostra quem parece ser o dono (ex.: Vercel) |
| `DEST_BUSY` | outro `skills` usando o destino | esperar |
| `LOCK_TOO_NEW` | lock gravado por versão mais nova | `mise upgrade github:matheusvcouto/cli-tools` |
| `CONFIG_INVALID` | `config.toml` inválido | campo, linha e motivo |
| `PROJECT_IS_GLOBAL` | `--project` aponta para a mesma pasta física do global | usar `--global` |

### 8.5 Marcadores de quem decide e sondas antes de construir

**Lá:** cada item da SPEC é marcado **[dono]**, **[eng]** ou **[validar]**;
`spec/decisoes-abertas.md` guarda as opções e a recomendação até você decidir;
a Fase 0 roda **sondas** manuais com agentes reais para responder os **[validar]**.

**No `skills`:** o plano marca cada decisão. Sondas manuais, antes do código,
**sem** usar suas pastas reais (só pastas de teste):

1. **[validar]** O Claude Code ignora um arquivo solto (`.skills-lock.json`) dentro de `skills/`?
2. **[validar]** O Codex lê `~/.agents/skills`, `~/.codex/skills` ou os dois? Nesta máquina
   existem as duas pastas com skills diferentes (`~/.codex/skills` tem 5).
3. **[validar]** O upload do Claude.ai aceita o `.skill` reprodutível (data fixa) e
   versão com pré-release no nome do arquivo?
4. **[validar]** O Claude Code e o Codex percebem uma skill atualizada sem reiniciar?

A resposta de cada sonda vai para o README antes da implementação.

### 8.6 Formato do plano

**Lá:** roteiro com fases, cada uma com **aceite observável**; só a fase atual
tem plano passo a passo ("escrever agora seria adivinhar"); cada tarefa lista
**arquivos**, **interfaces que produz**, **teste que falha primeiro**,
verificação e mensagem de commit; uma seção **"Foco de revisão"** nomeia os
riscos e o teste de cada um; tabela de **cobertura** liga cada caso de uso à fase.

**No `skills`:** o plano segue esse formato:

- Fase 0: sondas (§8.5) + esqueleto (`cmd/skills`, `tool.json`, teste de arquitetura).
- Fase 1: `core` (descoberta, especificação, hash, estados) + `list`, `status`, `doctor`.
- Fase 2: `dest` (lock, troca segura, backups de 30 dias) + `install`, `update`, `uninstall`, `diff`.
- Fase 3: `build` + `--all`/índice de destinos.
- Foco de revisão com teste nomeado para V1–V4, R1 e R6.

### 8.7 Cenários como dados

**Lá:** um agente falso guiado por **cenários em JSON**, reaproveitados por todas as suítes.

**No `skills`:** cenários em `testdata/` descrevem origem + destino + lock e o
resultado esperado (estado, ação, código de erro). Os mesmos cenários rodam no
`core`, no `status --json` e no `update --yes`. O que o README promete vira cenário.

### 8.8 Concorrência e escrita (SPEC §16.1)

**Lá:** trava de arquivo + temporário na mesma pasta + `fsync` + troca atômica +
`fsync` da pasta; versão antiga não escreve em formato novo (`STORE_TOO_NEW`).

**No `skills`:** lock do destino e índice em `state/` seguem exatamente isso, usando
`filelock` e `fscommit` da suíte. Lock com versão maior que a conhecida é só
lido, nunca reescrito (`LOCK_TOO_NEW`).

### 8.9 Relógio injetável e goroutines verificadas

**Lá:** `Clock` e `IDGen` injetáveis; `goleak` e `-race` nos testes.

**No `skills`:** `Clock` no `core` (a regra dos 30 dias é testada sem esperar
30 dias). A limpeza em goroutine tem teste provando que o comando espera por
ela e que o cancelamento (Ctrl-C) a interrompe sem apagar pela metade.
`goleak` só em teste: **[eng]** avaliar no ADR; sem ele, o teste usa um
`WaitGroup` exposto ao teste.

### 8.10 O que **não** copiar

- Schema-first com tipos gerados (ADR-008): faz sentido para várias linguagens;
  aqui há um consumidor só. O lock e o `--json` ganham um JSON Schema para
  **validar** nos testes (a suíte já tem `jsonschema`), sem geração de código.
- SQLite e serviço: um arquivo JSON por destino basta.
