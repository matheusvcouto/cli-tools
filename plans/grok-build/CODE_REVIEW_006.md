# SNAPSHOT-006 — Auditoria transversal: índice legado, seleção Windows e migração

**Data:** 2026-09-26. **Base examinada:** SNAPSHOT-005 (229 arquivos). **Módulo:** `github.com/matheusvcouto/cli-tools/v2`; Go mínimo `1.27.1`. **Escopo:** fluxo de `ai-profile` (Claude, Codex e Grok), carregamento/recuperação do índice, migração do formato NUON, resolução de providers Windows e impacto nos gates atuais de CI. Esta revisão não se apresenta como uma execução nativa do Grok/Windows.

## Achados e mudanças efetivas

| ID | Gravidade | Falha confirmada por leitura do código anterior | Ajuste implementado | Regressão incluída |
|---|---|---|---|---|
| S06-01 | Alta: integridade e disponibilidade | Quando `index.json` sumia **sem backup**, `refuseOrphanedProfileState` só identificava o padrão `tool-AAAAMMDDhhmmss-hex`. A migração oficial permite diretórios legados `claude-id`/`codex-id`; o estado anterior podia aceitar o root como vazio e perder a associação de aliases e credenciais. Quarentenas `.deleted-*` e NUON residual também eram ignorados. | Sem índice: recusar qualquer entrada residual, exceto `.index.lock`; preservar estado e exigir recuperação explícita. Reconhecimento de nomes gerados mantido apenas para diagnóstico específico. | `TestMissingIndexWithLegacyOrResidualEntryFailsClosed`; `TestMissingIndexWithOnlyLockIsEmpty`. |
| S06-02 | Alta: regressão potencial de migração | Aplicar o guard S06-01 sem alterar a migração bloquearia o cutover oficial, já que os perfis **existem fisicamente antes** de criar `index.json`, e o NUON original geralmente fica no mesmo root. | API separada `Store.ImportLegacy` sob lock real, não uma exceção aberta em `Update`: exige ausência de JSON/backup, perfil válido, diretórios reais imediatamente sob root, nenhum órfão, permite exatamente a origem explicitada se ela residir nesse root e revalida os bytes antes do commit. Não remove nem modifica o NUON original. | `TestMigratePreservesLegacyIndexInsideDestinationRoot`, teste de migração existente, `TestMigrateRefusesUnreferencedLegacyState`, `TestLegacyImportRequiresExactRealDirectories`, `TestLegacyImportRejectsExistingBackupWithoutIndex`, `TestLegacyImportRefusesChangedSource`. |
| S06-03 | Média: integridade/DoS da ferramenta de migração | `migrate-ai-profile-index` lia o índice NUON por `os.ReadFile` sem limite, apesar da fronteira de 8 MiB do store. | Leitura limitada a 8 MiB, somente arquivo regular, Lstat/handle/Lstat para identidade; revalidação da origem, inclusive externa, no commit sob lock. | `TestReadLegacyIndexRejectsOversizedInput` e fonte alterada S06-02. |
| S06-04 | Média: seleção incorreta no Windows | `resolveLaunch` fazia primeiro uma busca global por `grok.exe`/`codex.exe`/`claude.exe` e só depois buscava o comando original. Um `.exe` **de um diretório posterior** podia substituir silenciosamente o `.cmd` npm que o usuário priorizou no `PATH`. | Selecionar uma única instalação via `exec.LookPath(binary)` (ordem de diretórios `PATH` e extensões `PATHEXT` nativas). O `.cmd` conhecido continua passando pelo manifesto/entrypoint validado e `node.exe`, nunca por shell. Wrappers desconhecidos falham fechado. | `TestWindowsResolveLaunchHonorsPATHBeforeLaterExecutable` (execução exigida no Windows nativo). |
| S06-05 | Baixa: verificação de filesystem no bootstrap | O único arquivo ignorado durante a ausência do índice era `.index.lock`; sem verificar seu tipo, um lock não regular poderia resultar em leitura de store vazia. No Windows, `ReadDir` pode devolver a capitalização original de `.INDEX.LOCK`. | Ignorar o lock somente se for arquivo regular; comparar nomes reservados conforme semântica de identidade da plataforma, inclusive o nome do NUON explicitamente permitido. | `TestMissingIndexRejectsNonRegularLock` e gate Windows de migração/paths. |

### Análise transversal e conflitos evitados

- **Store e recuperação:** `Store.Load` continua não recuperando automaticamente backup anterior. Uma primeira transação falha ainda pode deixar apenas `.index.lock`, que não contém credenciais e não deve impedir bootstrap futuro. NUON, quarentena, temporários, diretórios gerados/legados e arquivos desconhecidos exigem inspeção humana quando `index.json` desaparece. Um índice **corrompido existente** nunca é tratado como ausência.
- **Importação legada:** a ferramenta `--from` identifica o NUON exatamente; a origem no mesmo root é permitida sem desativar a política normal contra órfãos. Um arquivo extra não citado pelo NUON bloqueia importação em vez de ser descartado. A origem fora do root também é relida para detectar alterações. Ainda não existe reparo automático nem inferência de aliases físicos.
- **Grok:** permanece com `GROK_HOME` por perfil, `config.toml` preservado, credenciais/overrides herdados seletivamente limpos e controles administrativos mantidos. ACP continua `grok agent <opções> stdio`, sem always-approve implícito. Essas mudanças de store/Windows se aplicam também a Claude e Codex. As instruções e ferramentas locais do projeto não são isoladas pelo wrapper.
- **Windows:** não foi alterado o Job Object, os locks nem o modelo de ACLs. A proteção de root com DACL herdável não reescreve ACEs **explícitas** de arquivos legados; este risco permanece um gate nativo. Não afirmar que `grok.exe` existe em Windows ARM64 só porque `ai-profile.exe` compila.
- **CI e release:** os jobs existentes já incluem `go test ./...` para as seis plataformas e o teste Windows específico. As novas regressões passam a pertencer a esses jobs, mas nenhum workflow foi disparado para este snapshot. O change record de patch de `ai-profile` foi acrescentado para o conjunto de arquivos de domínio alterados. Não afrouxar required checks.
- **View Limits:** continua **somente planejado**, com adapters read-only por provider/perfil e consentimento; não implementar leitura de tokens ou inferência de cota semanal/mensal sem contratos oficiais.

### Evidência disponível nesta rodada

A fonte da semântica de `LookPath`/`PATHEXT` é o código/documentação oficial do Go:
- https://go.dev/src/os/exec/lookpath.go
- https://go.dev/src/os/exec/lp_windows.go

Para o Grok, a posição dos argumentos ACP e a precedência de configuração continuam documentadas pelo upstream:
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md

**Limitação essencial:** o ambiente disponível tem Go `1.23.2`. A tentativa de obter `1.27.1` foi rejeitada pelo DNS/rede durante acesso ao `proxy.golang.org`. `go test`, `go vet`, builds cruzados ou nativos e testes de integração de provider **não foram executados** para este snapshot. Apenas verificações estáticas e de integridade de arquivos podem ser marcadas como executadas após os respectivos comandos locais.

### Gates objetivos ainda pendentes

1. Com Go `1.27.1`: `go test ./...`, `go vet ./...`, `go test -shuffle=on -count=3 ./...` e `./scripts/check-safe.sh all`, incluindo a ferramenta de migração e S06-01 a S06-04.
2. No Windows x64: os testes de `LookPath` com `.cmd` e `.exe` em diretórios diferentes; npm/pnpm oficiais instalados; LockFileEx, fscommit, Job Object, ACL de perfis novos e legados; caminhos Unicode/com espaços. No Windows ARM64: compilação/nativo do wrapper e disponibilidade real do Grok upstream verificada separadamente.
3. Grok oficial autenticado: dois perfis isolados, execução interativa/headless e ACP JSON-RPC com opções antes de `stdio`; políticas de permissão e cancelamento do processo.
4. GitHub Actions/release no **commit exato do snapshot**: seis targets e publicação dos mesmos bytes verificados. Não inferir sucesso de runs históricos.
5. Migração real: preservar backup externo completo de `~/.ai-profiles`, testar NUON **dentro** e **fora** do root; simular somente para fins de teste de regressão, nunca como substituto de dados reais. Conferir recuperação manual pós-falha e não excluir credenciais órfãs para silenciar o guard.

**Conclusão:** implementações adicionais reais foram necessárias e realizadas. O código está preparado para revalidação nativa, mas o status de produção ainda depende dos gates externos acima; nenhum resultado de runtime foi inventado.
