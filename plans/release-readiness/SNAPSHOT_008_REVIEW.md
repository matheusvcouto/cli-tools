# SNAPSHOT-008 — auditoria final antes de enviar para CI (`cli-tools`)

**Base:** SNAPSHOT-007. **Data:** 2026-09-26. **Módulo:** `github.com/matheusvcouto/cli-tools/v2`; **Go obrigatório:** `1.27.1`. **Conclusão diferenciada:** este código pode ser enviado **como PR/branch para executar os testes reais no GitHub** depois de uma inspeção local do diff. **NÃO criar a tag nem publicar release estável antes dos gates enumerados abaixo**. Nenhuma análise estática garante ausência absoluta de bugs.

## 1. Ajustes efetivamente implementados na revisão 008

| Achado | Risco concreto anterior | Correção | Evidência disponível |
|---|---|---|---|
| R08-01 — pré-validação tardia no release | Uma tag `v*` inválida ou um commit com `changes/*.json` pendente podia iniciar a matriz nativa de seis máquinas e instalação de shells antes de falhar no `build-release`. | Novo subcomando interno `go run ./tools/release preflight --version vX.Y.Z --changelog CHANGELOG.md` verifica changelog versionado, Go mínimo, caminho `/v2`, pacote das CLIs e ausência de records pendentes. Job `preflight` faz `actionlint` verificado, valida change records e exige que tag/commit remotos ainda sejam os esperados. `verify` **e** `native-shell-completion` dependem do job. O builder repete as verificações na fase de construção. | Código e grafo YAML inspecionáveis; execução real de Go/Actions pendente. |
| R08-02 — validações de tag inconsistentes | `publish-release.sh` e `verify-remote-release-tag.sh` aceitavam pré-release/build metadata (`-rc`, `+...`) e zeros à esquerda que `tools/release/main.go` recusava. | Os dois scripts agora aceitam apenas a mesma forma canônica estável `^v(0\|[1-9][0-9]*)\.(0\|[1-9][0-9]*)\.(0\|[1-9][0-9]*)$`. Não significa suporte a pré-releases: tal suporte exigiria um contrato único de versão, artefatos, notas e suíte. | `bash -n` e inspeção comparada; publicação no GitHub ainda não observada. |
| R08-03 — record symlink ocultado | A primeira versão da verificação pré-release ignorava `changes/pending.json` se fosse symlink/entrada não regular. | A verificação falha diante de **qualquer** entrada `*.json` no topo de `changes/`, independentemente do tipo, preservando `changes/archive/`. Testes de regressão adicionados para record regular, symlink, arquivo arquivado e diretório ausente. | Testes **escritos, não executados** aqui. |

Uma release deliberadamente **não está preparada** neste snapshot: permanecem vários change records. Isso é correto para um commit de desenvolvimento e garante que um push acidental de `v2.0.0` falhe no `preflight` em vez de publicar artefatos incorretos.

## 2. Reanálise dos cinco caminhos críticos

**`ai-profile` (Claude, Codex, Grok):** as rotas `list/new/rename/delete/run/acp` são despachadas por provider sem shell para argumentos de usuário. `DeleteConfirmedProfile` checa identidade completa sob lock antes de quarentena/commit, evitando reutilização concorrente do alias. O armazenamento usa `os.Root`, locking por SO, index/backups limitados, detecção fail-closed de órfãos e importação NUON explícita. `GROK_HOME`, `CODEX_HOME` e `CLAUDE_CONFIG_DIR` isolam *configurações próprias*, não arquivos do workspace nem credenciais de programas executados pelo agente. Grok chama ACP `grok agent <flags> stdio`, sem `--always-approve` implícito. Não encontrei outro erro determinístico demonstrável nesses caminhos na leitura da árvore 007; o risco residual é **comportamento de provedores reais**, auth e versões upstream (especialmente divergência documental de `compat.codex`).

**Windows:** código específico usa DACL protegida e herdável no root, `LockFileEx`, operações confinadas de commit, Job Object atribuído antes de `ResumeThread` e entrada npm oficial validada por `package.json`, acionada via Node sem `cmd.exe`. Essas construções respeitam as primitivas revisadas, mas um **arquivo de perfil legado com ACE explícita permissiva não é consertado somente pela DACL do root**. Requer inspeção nativa do security descriptor de arquivos legados. A CLI `ai-profile.exe` ARM64 pode compilar mesmo sem distribuição upstream do Grok nativo ARM64 — não confundir as duas garantias. Não foi possível executar Windows localmente.

**Migração:** `index.json` desaparecido com qualquer estado residual não é tratado como store vazio; importação e reconciliação exigem operação explícita. Não substituir por recuperação automática de backup sem confirmação. Aprovação exige diretório descartável, backup externo, casos `index.nuon` dentro/fora da raiz, interrupção/quarentena, identidades concorrentes e checagem das permissões resultantes no Windows.

**CI:** os runners `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15-intel`, `macos-15`, `windows-2025` e `windows-11-vs2026-arm` constam na documentação atual do GitHub; `setup-go` suporta leitura de `go.mod`. Runners existentes e YAML sintaticamente correto não demonstram disponibilidade dos programas npm nem sucesso do teste nativo. Os commits SHA fixados continuam protegendo contra movimentação das tags de Actions; cada download adicional (actionlint, Nushell) possui SHA-256 upstream verificado ou falha.

**Release:** build único em Linux com `CGO_ENABLED=0`, manifest SHA256, atestação no builder público, seis smokes nativos dos mesmos arquivos do artifact e publicação posterior de draft apenas após baixar e comparar novamente os bytes remotos. Reanálise da ordem dos jobs levou a `preflight` de tag/changelog/records/Actionlint antes de gastar seis runners. Riscos remotos não elimináveis localmente: permissões e regras do repositório, disponibilidade de `gh`, quotas de Actions, alteração da branch padrão, falha transitória de upload/download, revogação de Action upstream e contrato de APIs de release. Publicação deve ser observada em execução de tag realmente preparada.

## 3. Gates exigidos para promover a distribuição

**Etapa A — enviar esta revisão como PR/branch sem tag:** confirmar que `git diff --check`, `gofmt -l .`, JSON/TOML/YAML e `bash -n` passam no computador. O CI real deve concluir `workflow-lint`/actionlint, `change-records`, `contract-locks`, `public-go-api`, seis jobs `test` (incluindo `-race` suportado) e `native-shell-completion`. Avaliar primeiro logs específicos de Windows x64/ARM64 e a disponibilidade do toolchain Go 1.27.1. Registros antigos de CI não contam para este commit.

**Etapa B — integração real antes de dizer funcional para uso geral:** em máquinas compatíveis, autenticar pelo menos duas identidades descartáveis em cada provider; verificar que rodar/ACP alternados não herdaram a credencial da outra; stdout ACP deve conter somente o protocolo; cancelar e conferir cleanup da árvore de processos; conferir npm/pnpm em Windows; auditar ACEs explícitas em perfis Windows legados. Testar restauração/migração sobre cópia com backup externo. Documentar versões exatas dos CLIs Claude, Codex e Grok testadas.

**Etapa C — preparar e etiquetar apenas após A e B:** na branch com todos os gates aprovados, executar `go run ./tools/release prepare --suite-version v2.0.0` para preview, depois `... --write`, revisar os changelogs/manifests/arquivamento dos change records e **commitar**. Rodar CI novamente nesse commit preparado. Conferir `go run ./tools/release preflight --version v2.0.0 --changelog CHANGELOG.md` e `go run ./tools/release changes validate`. Criar e enviar tag protegida somente nesse commit; observar `preflight -> verify + native-shell-completion -> build-release -> smoke Unix/Windows -> publish` e conferir hashes dos downloads públicos. Um workflow de tag falhando deixa a release **não publicável**; não alterar os gates para fazê-lo passar.

### Notas sobre prova

Testes adicionados ao repositório não são provas de execução. Nesta rodada foram reanalisados os caminhos acima e realizadas as verificações estáticas que o ambiente permite; **Go 1.27.1 e as execuções reais dos provedores e runners continuam ausentes**. A tentativa de baixar Go 1.27.1 foi recusada pelo ambiente e não houve fallback para Go 1.23.2.

## 4. Documentação primária consultada

- Go 1.27.1: https://go.dev/dl/ ; toolchain: https://go.dev/doc/toolchain ; `os.Root`: https://pkg.go.dev/os#Root
- GitHub runner labels: https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- `setup-go` com `go.mod`: https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md
- Actions push/tag event: https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
- Segurança das Releases/Artifacts: https://github.com/actions/attest ; https://cli.github.com/manual/gh_release_create ; https://cli.github.com/manual/gh_release_download
- Grok ACP: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md ; configuração: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/26-config-reference.md
- Win32 locks/DACL/Job Objects: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex ; https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-setsecurityinfo ; https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects

**View Limits** permanece projeto futuro no plano separado, não acessa credenciais nem cotas nesta revisão.

## 5. Resultado concreto da validação estática de 008

- `gofmt -l .`: **PASS**, 119 arquivos `.go` presentes, sem alterações pendentes; sintaxe dos cinco scripts `bash -n`: **PASS**.
- Parsing dos 15 JSON e TOML presente: **PASS**; `PyYAML.safe_load` de `ci.yml` (7 jobs) e `release.yml` (7 jobs), referências `needs` existentes, grafo sem ciclos, `uses` externos em SHAs de 40 caracteres, seis plataformas `verify`, quatro + dois smokes e `publish` dependente dos dois: **PASS estrutural**. **`actionlint` oficial ainda não foi executado aqui**; a verificação estrutural não o substitui.
- Verificação estática dos diretórios de imports internos Go: **PASS** (119 fontes, com a ressalva de que não há type-check). Regex canônico executado no Bash real para versões válidas e malformadas: **PASS**.
- `GOTOOLCHAIN=local go test ./...`: **BLOQUEADO antes de executar testes**; comando retorna `go.mod requires go >= 1.27.1 (running go 1.23.2; GOTOOLCHAIN=local)`. Tentativas de obter binário oficial neste ambiente falharam; não foi executado `go vet` nem ação remota. Os testes `preflight_test.go` são novos e aguardam execução verdadeira no Go 1.27.1.
