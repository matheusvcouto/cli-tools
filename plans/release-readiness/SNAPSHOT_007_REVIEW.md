# SNAPSHOT-007 — revisão para uso real (`cli-tools`)

**Data:** 2026-09-26. **Base:** SNAPSHOT-006; repositório completo preservado. **Módulo:** `github.com/matheusvcouto/cli-tools/v2`; **Go mínimo:** `1.27.1`. **Avaliação:** código implementado e fortalecido; **não há evidência para declarar release pronta para produção em todas as seis plataformas**. Nenhuma execução bloqueada foi substituída por mock ou aprovação artificial.

## 1. Achados reproduzíveis na leitura do código e ajustes

| ID | Risco antes da correção | Alteração implementada | Regra de regressão (escrita; execução pendente) |
|---|---|---|---|
| S07-01 (alta; exclusão de diretório errado) | O `ai-profile <provider> delete <alias>` lia o perfil e mostrava seu caminho antes de dois prompts. Depois chamava `DeleteConfirmed(tool, alias)`, que resolvia de novo o alias sob lock, permitindo excluir **outro diretório** se outro processo renomeasse o primeiro perfil e reutilizasse o alias enquanto o usuário confirmava. | A CLI passa o perfil **completo exibido** a `DeleteConfirmedProfile`. `Store.deleteProfileIfUnchanged` compara `Tool`, `Alias`, `Dir` e `CreatedAt` sob o mesmo lock que protege quarentena e commit. Mudança detectada cancela a exclusão e exige nova confirmação. A API explícita `DeleteConfirmed(tool,alias)` permanece para consumidores que não possuem uma seleção interativa anterior. | `TestDeleteConfirmedProfileRejectsReusedAlias`, `TestDeleteConfirmedProfileRejectsChangedMetadata`, `TestInteractiveDeleteRefusesProfileReplacedDuringConfirmation`. |
| S07-02 (média; execução pós-cancelamento) | O runner Unix descartava o `context.Context` antes de `syscall.Exec`; uma solicitação já cancelada ainda podia iniciar o provider. O runner Windows também resolvia o binário antes de conferir cancelamento. | Conferir `ctx.Err()` antes de resolução em ambos os SOs; Unix confere novamente antes do `syscall.Exec`. **Limitação explícita:** contexto não pode cancelar um processo após o sucesso de `syscall.Exec` — seu ciclo de vida então segue a semântica Unix normal. | `TestRunnerDoesNotExecAfterCancellation`, `TestWindowsRunnerDoesNotLaunchAfterCancellation`. |

A lógica de exclusão segura sob lock foi implementada em `internal/aiprofile/store.go`; a CLI em `internal/aiprofile/cli/app.go`, e os testes estão nos respectivos `_test.go`. Os contratos públicos de comandos (`list/new/rename/delete/run/acp`) não mudaram. Os arquivos de mudanças registram patch `ai-profile`; não etiquetar `v2.0.0` antes de `release prepare --write` consumir todos os change records pendentes.

## 2. Reanálise estática de outras áreas e riscos residuais

- **Grok**: `grok agent <opções> stdio` confere com a documentação oficial de ACP. `GROK_HOME` é individual por perfil; os novos perfis começam com compatibilidade Claude, Cursor e Codex desativada. Há **divergência entre as duas páginas do próprio upstream**: `05-configuration.md` diz que `compat.codex.skills/hooks` são reservados/inertes, enquanto `26-config-reference.md` os lista como fontes de scan. Manter `false` é conservador, mas a eficácia no executável de cada versão exige `grok inspect` real. CLI não ativa always-approve. Regras corporativas e configurações do projeto podem continuar incidindo; `GROK_HOME` **não** é sandbox.
- **Claude/Codex**: domínios de configuração separados; variáveis de autenticação de cada provider são filtradas, mas credenciais de ferramentas invocadas pelo agente podem continuar disponíveis por desenho. Não prometer isolamento absoluto de `$HOME` ou dos arquivos de projeto.
- **Store/migração**: esquema JSON, backups anteriores ao último commit, leitura/gravação limitada a 8 MiB, locks reais por plataforma, `os.Root`, importação NUON explícita. Não restaurar automaticamente `index.json.bak` sem conciliação; em caso de perda de índice, preservar o root inteiro e seguir recuperação manual. A migração com dados do usuário só pode ser considerada aprovada após execução real com backup.
- **Windows**: LockFileEx, root DACL, commit e Job Object estão implementados, mas ACLs **explícitas** de arquivos legados podem permanecer; isso exige validação em Windows nativo. `.cmd` npm é resolvido para Node a partir de package manifest verificado, sem shell; cobertura npm/pnpm reais e Grok Windows ARM64 **não comprovadas**.
- **GitHub Actions/release**: matriz de seis targets, build once, checksum e smoke da distribuição antes de publicação são coerentes na análise do YAML. Os checks de PR/merge_group existem. Sem disparar o workflow **nesse commit** não se pode afirmar que os jobs passam; runs antigos são evidência somente da revisão antiga.
- **View Limits**: permanece apenas planejado em `plans/view-limits/README.md`. Não há coleta automática de credenciais ou de limites nesta versão.

## 3. Critérios antes de instalar ou liberar para uso

**Não publicar como versão final multiplataforma até verificar:**

1. Linux/macOS com Go `1.27.1`: `go version`, `go test ./...`, `go vet ./...`, `go test -shuffle=on -count=3 ./...`, `./scripts/check-safe.sh all`; gate de race onde suportado.
2. GitHub Actions **na revisão correspondente ao snapshot 007**: `workflow-lint`, `change-records`, `contract-locks`, `public-go-api`, matriz de testes em seis plataformas e release smoke do mesmo binário arquivado. Validar preparação do `v2`/tag com change records consumidos antes de disparar release.
3. Testes no Windows x64/ARM64: DACL de root e perfis antigos, locks entre processos, commit/rollback e `PATH`/`PATHEXT` em instalações npm/pnpm de cada provider; não presumir disponibilidade de Grok ARM64 por causa do binário `ai-profile` ARM64.
4. Integrações autenticadas **reais**: Claude Code, Codex e Grok com dois perfis cada; confirmar troca/ausência de vazamento de credenciais, login, `run`, ACP com stdin/stdout binários sem logs na saída, cancelamento e saída do filho.
5. Migração real: backup externo de `~/.ai-profiles`, NUON dentro e fora do root, exclusões interativas e recuperação de quarentena após falha. Testar em dados descartáveis antes de mexer no armazenamento real.

**Escopo de uso atual:** comandos estáticos `--help`, `--version` e consulta de contrato são candidatos a piloto após build real. Perfis novos/migração/exclusão e ACP devem ser tratados como **pré-release até os gates acima**. Não é tecnicamente possível garantir ausência de bugs futuros mesmo após todos os testes.

## 4. Verificação externa consultada

- Go `1.27.1` oficial: https://go.dev/dl/ e https://go.dev/doc/devel/release
- `setup-go` lê o `go.mod`: https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md
- Grok ACP: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- Grok configuração (`05`, potencialmente desatualizada em Codex): https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md
- Grok referência detalhada (`26`): https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/26-config-reference.md

## 5. Evidência e restrições da auditoria local

**Verificações estáticas locais aprovadas:** `gofmt` em 117 arquivos Go (zero arquivos pendentes), inspeção de 70 referências de imports internos (zero paths ausentes e zero imports sem `/v2`; sem type-check por falta do toolchain), parse de 14 JSON, 2 workflows YAML com 13 jobs, 1 TOML e `bash -n` de cinco scripts. O pacote contém 233 arquivos: o `unzip -t`, a comparação integral da árvore extraída e a reconstrução Base64 produziram resultados consistentes. **Não executados:** `go test`, `go vet`, cross-build com Go `1.27.1`, Grok autenticado, ACP, Windows/macOS nativos e GitHub Actions desta árvore. O sandbox possui somente Go `1.23.2`; `GOTOOLCHAIN=local go test ./...` encerrou sem executar a suíte porque `go.mod` exige Go >= `1.27.1`, e a tentativa de acesso a `go.dev` falhou por DNS. Esses gates permanecem pendentes, não dispensados.
