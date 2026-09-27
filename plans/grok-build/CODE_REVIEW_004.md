# SNAPSHOT-004 — Auditoria transversal cli-tools / Grok Build

**Data:** 26/09/2026. Base: SNAPSHOT-003. Módulo: `github.com/matheusvcouto/cli-tools/v2`, Go mínimo **1.27.1**. Este documento separa **ajuste implementado**, **revisão estática** e **gate de execução ausente**. Não afirma compatibilidade de produção que o ambiente não comprovou.

## Correções incluídas

| ID | Gravidade | Falha/riscos concretos | Ajuste | Como verificar |
|---|---|---|---|---|
| S04-01 | Alta | Índice válido por alias, mas com **dois perfis apontando para o mesmo diretório**: `delete` podia apagar dados da outra identidade. | `Store.validate` exige propriedade exclusiva de diretórios para todos os providers. Normalização separada Unix/Windows (`filepath.Clean`, case-fold Windows) antes de carregar ou gravar o índice. Testes cobrem mutação e índice manual inconsistente, inclusive recusa de exclusão. | `go test ./internal/aiprofile -run 'Test(StoreRejectsAliasedProfileDirectory|DuplicateDirectoryInIndexBlocksDelete|ProfileDirIdentityWindowsCaseFold)'` nos runners adequados. |
| S04-02 | Média | `index.json`/backup e `settings.json` eram lidos com `io.ReadAll` sem limite: arquivo truncado/corrupto/muito grande podia exaurir RAM em `list`, `run`, `new` e recovery. | Teto de 8 MiB antes e durante leitura, aplicado aos helpers com `os.Root` e caminho isolado, falha fechada antes de decodificar. Testes de arquivo sparse e leitura no limite. O tamanho não foi imposto ao arquivo de credenciais do Grok. | Testes `TestOversizedProfileIndexFailsClosed` e `TestBoundedMetadataReadRejectsGrowthBeyondLimit`. |
| S04-03 | Média | Shell pai podia injetar destino de **feedback** ou região de trace do Grok em perfil selecionado. | `GROK_FEEDBACK_BASE_URL` e `GROK_TRACE_UPLOAD_REGION` adicionados à remoção seletiva. Flags de segurança corporativa (`GROK_DISABLE_API_KEY_AUTH`, `GROK_FORCE_LOGIN_TEAM_ID`, sandbox, `GROK_REQUIRED_*`) permanecem herdadas. Testes ampliados. | Teste de isolamento do ambiente e `grok inspect --json` em versão oficial. |
| S04-05 | Média | Um index ou settings criado pelo próprio aplicativo podia ultrapassar o limite de leitura recém-introduzido, fazendo a operação seguinte falhar apesar de um commit bem-sucedido. | Imposto o teto de 8 MiB também à gravação de index, backup e Claude settings; o index excedente é recusado antes da escrita do backup. Testes verificam que o índice e o settings anteriores permanecem íntegros. | `TestOversizedGeneratedIndexIsNotCommitted` e `TestOversizedMetadataWritePreservesExistingFile`. |
| S04-04 | Planejamento | View Limits futuramente precisa consultar saldo por conta/perfil sem confundir assinatura e API nem vazar segredo. | Plano de interfaces, consentimento, source provenance e gates em `plans/view-limits/README.md`. **Nenhum endpoint de quota foi implementado.** | Futuros gates VL-01..VL-05 independentes de ACP. |

## Áreas revisadas

- **Modelo/store:** validação de aliases, diretórios, JSON com `DisallowUnknownFields`, backup, lock cross-platform, rollback de criação, quarantine de exclusão e `os.Root`. O ajuste S04-01 evita exclusão cruzada num índice de alias válido porém inconsistente; S04-02 trata os reads ilimitados e S04-05 impede writes que tornariam metadados ilegíveis. Não resolver problemas graves sobrescrevendo um índice corrupto automaticamente.
- **Grok:** criação transacional, `GROK_HOME`, preflight de `config.toml`, variável de autenticação/endpoint/compatibilidade e política de segurança; ACP `grok agent <opções> stdio` com stdio limpo. Os dois novos overrides removidos vêm da referência oficial atual; não fazer remoção genérica de `GROK_*`.
- **Windows:** adapter separado, preferência por `grok.exe`, npm `@xai-official/grok` com manifesto/entrypoint validado e Node direto sem shell; lock, substituição e processos com Job Object continuam dependendo de validação Windows real. `profileDirIdentity` usa case-fold somente no Windows.
- **CI/release:** workflows têm jobs para seis targets, verificação de release build-once, checksums e smoke do mesmo artifact, `change-records` na Merge Queue, actions pinadas. Revisão estrutural local não garante disponibilidade/permissões reais dos runners nem passa por `actionlint` remoto.
- **Contratos e documentação:** `cmd/ai-profile/cli.contract.json` já contém Grok; `cli/api.contract.json` é contrato da biblioteca `cli` e corretamente não lista providers. View Limits não foi adicionado prematuramente a ambos.

## Limites da revisão — não marcar como PASS

- Go local **1.23.2**, módulo requer **1.27.1**: `go test`, `go vet`, `go run ./tools/release ...` e compilação nativa não podem ser concluídos aqui sem toolchain correto. Não editar `go.mod` para fabricar evidência.
- Não houve execução do **Grok oficial autenticado**, validação de dois logins reais, ACP JSON-RPC real nem `grok inspect --json` autenticado.
- A DACL protegida do root no Windows **não elimina ACE explícita preexistente** em `auth.json`, `config.toml`, `index.json` ou outros arquivos legados. Testar descriptors nativos e outro usuário; desenhar migração de ACL segura separadamente. Não alegar proteção retroativa total.
- Simultaneidade de `run` + ACP sob um `GROK_HOME` e interações com políticas empresariais não foram verificadas com upstream. Bloqueios externos não autorizam mocks apresentados como prova.
- Os perfis ficam isolados quanto ao home e overrides conhecidos; **não** são sandbox para contexto/projetos, MCPs, hooks, credenciais genéricas ou política do próprio Grok.
- Índices com links/junções criados maliciosamente **pelo próprio usuário** após a validação, aliases físicos de Win32 com reparse points ou alteração da configuração em memória por outro processo continuam fora de uma garantia de isolamento entre processos do mesmo usuário.

## Checklist para promover a compatibilidade

1. Checkout do commit exato em máquina com Go 1.27.1 oficial; executar `./scripts/check-safe.sh all`, `go vet ./...`, testes de shuffle/race adequados ao target e comandos `contracts check` / `api check`.
2. Executar GitHub Actions CI e release dry-run em runners oficiais Linux/macOS/Windows x64/ARM64. Windows ARM64 do **wrapper** não constitui evidência do binário **Grok** ARM64.
3. Instalar Grok oficial versionado, identificar hash/distribuição, testar login de dois perfis, TUI/headless/ACP, permissões, cancelamento, `--model`, Unicode e paths com espaços no Windows x64.
4. Windows: verificar permissões explícitas de arquivos **legados**, novo root, User/SYSTEM/Administrators, acesso negado a outro usuário, lock/process tree, backup/replace e limites de leitura.
5. Só declarar supported depois da evidência com resultados íntegros dos gates reais. Se upstream divergir, ajustar adapter à versão real.

## Fontes oficiais reconsultadas

- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/26-config-reference.md (env `GROK_FEEDBACK_BASE_URL` e `GROK_TRACE_UPLOAD_REGION`, camadas e pins de política)
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md (ACP/ordem das opções)
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/02-authentication.md (credenciais sob Grok home)
- https://docs.github.com/en/actions (runners, jobs, artefatos e release; validação efetiva no GitHub permanece pendente)
- https://pkg.go.dev/os (Root/os.File e limites das verificações por identidade de arquivo)

## Evidência exata desta rodada

- `GOTOOLCHAIN=local ./scripts/check-safe.sh fmt`: **PASS**; `gofmt -l .`: sem arquivos pendentes; `bash -n scripts/*.sh`: **PASS**.
- Python: parse de **12 JSON**, `grokDefaultConfig` via `tomllib`, **2 YAML** com **13 jobs**; dependências `needs` internas verificadas; `uses:` não locais fixados em SHA completo.
- `GOTOOLCHAIN=local go test ./...`: **BLOQUEADO**, Go 1.23.2 em vez de 1.27.1 (erro real de requisito do `go.mod`); download do toolchain também falhou por DNS. `go vet`, Windows e upstream Grok **não executados**. `actionlint` não disponível localmente.
- Validação da árvore arquivada, contagem final, `unzip -t`/equivalente `zipfile.testzip`, SHA-256 e reconstrução Base64: registrar somente após execução bem-sucedida, no resumo de entrega. **Testes adicionados não contam como testes executados quando o Go exigido não está instalado.**
