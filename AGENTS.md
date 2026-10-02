# AGENTS.md

Regras canônicas para qualquer agente que trabalhe neste repositório.

## 1. Contexto mínimo

Leia sempre:

1. `README.md`
2. `ADR.md`
3. `docs/engineering.md`
4. `docs/architecture.md`
5. `docs/testing.md`
6. `docs/platforms.md`
7. `docs/portability.md`

Ao trabalhar em uma CLI existente, leia também o ADR específico:

- `cmd/ai-profile/ADR.md`;
- `cmd/repo-zip/ADR.md`.

Se `plans/` contiver um plano ativo relacionado à tarefa, leia o `README.md` dele e siga sua ordem/checklist.

`docs/history/` é histórico: não deve ser carregado por padrão. Consulte apenas para regressão, auditoria ou decisão antiga específica.
Relatórios datados de revisões em `plans/` também são evidências da rodada em
que foram escritos. Referências antigas a snapshots, /v2, releases ou falta de
Go/rede não autorizam mudanças de contrato nem substituem o estado vigente em
go.mod, manifests, docs ativas e CONTEXT.md.

## 2. Restrições inegociáveis

- Nunca testar contra estado real do usuário.
- Nunca escrever, renomear ou apagar `~/.ai-profiles` em testes ou durante a implementação da migração.
- Nunca executar `claude`, `codex`, `grok`, `claude-agent-acp` ou `codex-acp` reais em testes.
- Nunca acessar Keychain, Credential Manager, Secret Service, tokens ou contas reais.
- Nunca alterar Git config global/sistema.
- Repositórios Git de teste devem existir somente sob diretório temporário controlado pelo teste.
- A execução da suíte padrão não depende de rede. Preparação de toolchain e
  dependências pode usar rede antes dos testes; essa preparação não é um teste.
- Não copiar para fixtures/logs dados reais e depois “redigir”; fixtures já nascem sintéticas.
- Não commitar, pushar, publicar release ou alterar configuração real do usuário sem pedido explícito.

## 3. Arquitetura da suite

- Um único módulo Go.
- Todo binário distribuído fica em `cmd/<nome>/`, possui `tool.json` próprio e usa o CLI Core declarativo em `cli/`.
- `--version` mostra a versão individual do produto; a tag da suíte/módulo é independente e aparece em `version --json`.
- Parsing, help, completion, schema, docs e contract devem derivar do mesmo `CompiledApp`; não manter command trees paralelas.
- Comandos, flags e argumentos usam stable IDs. `__cli` é namespace reservado do core.
- Help/version/schema/contract/completion estática não podem inicializar store/HOME/dependências de domínio.
- Regra de negócio de cada CLI fica em `internal/<dominio>/`.
- Regra de negócio não pode depender de `runtime.GOOS`, build tags, Win32/POSIX ou utilitário específico do SO; use ports/backends conforme `docs/platforms.md`.
- Plataforma/capability não implementada deve retornar erro explícito; é proibido fallback silencioso menos seguro.
- `cmd/*` contém apenas entrypoints distribuíveis. Ferramentas internas de build ficam em `tools/`, nunca em `cmd/`.
- Não criar `utils`, `helpers`, `common` ou `shared` genéricos.
- Só extrair pacote reutilizável quando houver semântica comum comprovada, não apenas chamadas parecidas à stdlib.
- Nova CLI deve seguir `docs/adding-tools.md`.
- `cli/` é a exceção pública aceita. Outro pacote Go público fora de `internal/` exige consumidor externo real e registro no ADR do escopo afetado.

## 4. Segurança e filesystem

- Nunca validar containment com prefixo textual.
- Não tratar a grafia absoluta de um path como identidade única. Em macOS,
  `/var/...` e `/private/var/...` podem apontar para o mesmo objeto. Quando os
  dois objetos existentes devem ser o mesmo arquivo/diretório, comparar
  `os.Stat` + `os.SameFile`; quando symlink em si importa, usar `Lstat` e manter
  a política explícita. `filepath.Abs`, `Clean` ou igualdade de strings não
  provam identidade física.
- Igualdade textual de paths só é válida quando a grafia é parte deliberada do
  contrato. Containment continua sendo validado com `filepath.Rel`/APIs
  confinadas, nunca com `os.SameFile` isoladamente.
- Preferir APIs confinadas (`os.Root`) quando a operação deve permanecer dentro de uma raiz.
- Usar `Lstat` quando symlink não deve ser seguido.
- Nunca assumir que `os.Rename` oferece a mesma atomicidade em todos os SOs.
- Build/cross-build verde não significa suporte de runtime; só declarar `supported` após teste real da capability naquele SO.
- Operação destrutiva só remove/quarentena objetos que foram validados como pertencentes à raiz controlada.
- Temporário destinado a publicação deve ficar no mesmo filesystem/diretório lógico do destino.
- Nunca apagar um arquivo de destino para “facilitar” replace; usar primitiva de commit apropriada e testada.

## 5. Processos

- Construir argv como lista; sem shell intermediária.
- ACP reserva stdout integralmente ao processo/protocolo filho.
- Logs/diagnóstico do wrapper vão para stderr.
- Variáveis de autenticação a limpar são removidas do ambiente, não preenchidas com string vazia.
- Testes de subprocesso usam fakes cujo caminho resolvido é comprovadamente temporário antes do spawn.

## 6. Dependências

Política: **stdlib-first, não stdlib-only**.

Dependência externa só entra quando torna a implementação comprovadamente mais segura/correta/manutenível. Antes de adicionar:

1. provar a lacuna concreta;
2. avaliar licença/manutenção/transitivas;
3. preferir dependência pequena e focada;
4. registrar a decisão no ADR da suíte ou da CLI afetada;
5. adicionar teste que cubra a semântica motivadora.

Gerenciar módulos com `go get pacote@versão`, `go mod tidy` quando necessário e
`go.mod`/`go.sum`. Preparar as dependências declaradas com
`./scripts/check-safe.sh prepare`, reutilizando `dist/go-cache/` (ignorado).
O Go compatível deve estar no PATH antes da preparação. Downloads públicos
são permitidos nessa etapa; manter validação de integridade habilitada.
Não criar `vendor/` por causa de uma limitação antiga do ambiente. Vendoring
exige uma necessidade própria, documentada, e não é o fluxo padrão.

`golang.org/x/sys` é candidato aceitável para primitivas nativas de lock/replace, se necessário. Não adicionar Cobra/Viper apenas por conveniência.

## 7. Release e versionamento

- Há três contratos: tag da suíte/módulo, versão individual em `cmd/<tool>/tool.json` e inteiros de protocol/schema/contract. Não os sincronizar artificialmente.
- Mudança relevante recebe `changes/*.json`; CI valida cobertura por componente. Não fazer bump manual por commit.
- `go run ./tools/release prepare --suite-version X.Y.Z` é preview; `--write` materializa manifests/changelogs/contracts e arquiva records.
- `cli/` é API Go pública reutilizável; `cli/api.contract.json` protege a superfície exportada e `go run ./tools/release api check` é gate obrigatório antes de release.
- API aditiva exige `api write` para passar a ser protegida; `api write --allow-breaking` exige quebra deliberada e change record de `module`.

### Identidade do módulo — não alterar automaticamente

- O caminho canônico é exatamente `github.com/matheusvcouto/cli-tools`.
- Não acrescentar `/v2`, `/v3` nem qualquer `/vN` ao módulo ou aos imports.
  Não renomear o módulo, criar outro módulo versionado ou reescrever imports
  como parte de manutenção, novas CLIs, dependências, refatoração ou release.
- Atualizar a suíte ou um produto não autoriza mudar o caminho do módulo.
  Versões como `1.3.1` e `1.3.2` permanecem no caminho canônico sem sufixo.
- Plano, ADR, snapshot, change record, documentação antiga, recomendação de
  ferramenta ou inferência do agente nunca autorizam essa migração.
- Uma migração de major/path só pode ser executada após pedido explícito do
  usuário que autorize essa mudança específica. Pedidos genéricos como
  “atualize”, “prepare a release” ou “siga boas práticas” não a autorizam.
- Se uma alteração pretendida exigir quebra da API Go pública, preservar a
  compatibilidade quando possível. Caso a quebra seja indispensável, explicar
  o motivo e os caminhos afetados e obter a decisão explícita do usuário antes
  de mudar major, go.mod ou imports. Não publicar uma versão incompatível com
  o caminho atual nem enfraquecer os gates para contornar a necessidade.
- Saltos da suíte e abandono da próxima versão calculada também exigem pedido
  explícito; nunca escolher uma mudança drástica de versão por conta própria.
- Se a versão calculada/proposta divergir da sequência esperada ou pedida pelo
  usuário, perguntar antes de prepare --write, commit de preparo, tag ou release.
  Informar resumo das mudanças, versão atual/esperada/proposta da suíte, versões
  dos produtos e motivo da divergência; aguardar aprovação explícita da versão.
  Aviso em commentary, preview e autorização genérica de publicar não bastam.
- Go mínimo continua `1.27.1`; não rebaixar a toolchain para passar em sandbox.

### Demais garantias de release

- Produto que cruza para `1.x` precisa declarar `stability: "stable"` no change record; estabilidade nunca pode regredir.
- `release prepare --write` deve preservar rollback do conjunto em qualquer erro retornado; não reintroduzir mutações parciais sem teste de restauração.
- Tags estáveis usam exatamente `vX.Y.Z`, nunca são reutilizadas/movidas e só apontam para commit já preparado com CI verde.
- GitHub Release usa a seção versionada do `CHANGELOG.md`; release notes automáticas não substituem o histórico canônico.
- Tooling recusa versão/changelog inválidos e output não vazio; nunca limpa recursivamente caminho fornecido ao comando.
- `SHA256SUMS` é relativo ao seu diretório e deve ser verificado com cwd nele.
- Smoke de release compara `--version` com `tool.json` e `version --json .suite_version` com a tag.
- Assets mise são validados em HOME/MISE_*/GH_CONFIG_DIR isolados antes de declarar a release pronta.

## 8. Qualidade

Conforme aplicável, prefira o runner sandboxed:

```sh
./scripts/check-safe.sh all
```

`mise run check` é conveniência para uso interativo, mas o mise pode descobrir
configuração global antes de chamar a task. Agentes que precisam provar
isolamento devem executar `scripts/check-safe.sh` diretamente.

Execute `./scripts/check-safe.sh prepare` antes dos checks quando houver
novas dependências. Os subcomandos `fmt`, `test`, `vet`, `shuffle`, `race` e
`fuzz` usam HOME/TMP/configuração de aplicação descartáveis e caches Go
persistentes dedicados em `dist/go-cache/`. Não herdam secrets/configurações
Git do usuário e bloqueiam downloads de módulos/toolchain durante os checks.
Isso não constitui bloqueio integral da rede de processos de teste.
Fuzz roda na worktree persistente para preservar casos de falha em testdata.
Caches/builds são regeneráveis; casos de falha descobertos são evidências e
não devem ser apagados automaticamente.

No macOS, testes herméticos devem priorizar o Apple Git real em
`/Library/Developer/CommandLineTools/usr/bin` quando ele existir. O executável
`/usr/bin/git` pode ser um shim do Xcode e emitir diagnóstico ao resolver as
Command Line Tools sob um ambiente mínimo. Quando stdout possui valor
estruturado, como hash ou ref, capture stdout e stderr separadamente; nunca use
`CombinedOutput` como o valor a ser parseado.

`-race` é um job separado: não force `CGO_ENABLED=0` nele. Builds de release devem permanecer `CGO_ENABLED=0`, salvo ADR explícito.

Se algo não pôde ser executado, registrar **não executado**; nunca chamar de aprovado.

Ao corrigir uma falha de CI, localizar o teste pelo nome completo mostrado no
log e revisar o diff no contexto dessa função. Se uma expressão idêntica existir
em vários testes, não assumir que a primeira ocorrência é o alvo. Uma correção
de portabilidade só está comprovada depois do job nativo que revelou o problema.

Testes de instalação mise devem usar `MISE_NO_CONFIG=1`/`--no-config`, todos os
diretórios `MISE_*` e `GH_CONFIG_DIR` sintéticos, além de desativar fallbacks de
tokens/credenciais. Redirecionar somente HOME/MISE_* não prova que o mise deixou
de descobrir configuração pessoal.

## 9. Definition of done

Uma mudança só está pronta quando:

1. comportamento/contrato está claro;
2. implementação é mínima e legível;
3. testes relevantes passam;
4. falhas e rollback foram testados quando há mutação;
5. docs/ADR foram atualizados quando necessário;
6. nenhum estado real foi tocado;
7. não há afirmação de suporte sem evidência correspondente.

Durante uma migração ativa, atualizar também o checklist indicado pelo `README.md` daquela migração.

## 10. Regras adicionais de evidência

Não gerar snapshots ZIP, Base64, checksums de snapshot nem cópias separadas
para entrega. A entrega normal é a alteração na worktree persistente, com diff,
resultados dos checks e builds locais quando uma CLI foi alterada. Artifacts
antigos permanecem preservados e ignorados; archives/checksums de release e
fixtures de ZIP continuam sendo parte do produto, não snapshots de entrega.

- Nunca inventar gates, testes, evidências ou resultados CI. Revisão estática/documentação oficial é evidência **de revisão**, não de runtime.
- Não degradar produção, garantias fail-closed ou baseline Go para adaptar ao ambiente. Verificar as capacidades atuais antes de registrar bloqueios.
- Ao finalizar um ponto, continuar para o próximo quando tecnicamente possível; registrar bloqueios e validações externas pendentes no `CONTEXT.md`.

## Grok Build — contrato e segurança

- O provider `grok` está implementado; ler `plans/grok-build/README.md`, `REPORT_AND_PLAN.md` e `VALIDATION.md` antes de alterar seu contrato.
- A suíte padrão permanece offline e não deve executar `grok` real nem tocar `~/.grok`, tokens, MCPs, projetos ou contas reais do desenvolvedor.
- Probes sintéticos validam somente o launcher (argv/env/exit code); não são prova de compatibilidade Grok.
- ACP Grok é `grok agent <opções> stdio`; não mover opções após `stdio`.
- Não injetar `--always-approve`/`--yolo`; preservar deny rules, hooks, folder trust e sandbox.
- No Windows, nunca executar `grok.cmd` por `cmd.exe`. Resolver somente o package oficial com manifesto/entrypoint verificado e Node direto, ou preferir `grok.exe`.
- Não limpar `GROK_*` genericamente: isso pode remover guardrails administrados. Alterações na lista de ambiente exigem reconsulta às docs atuais.
- Execuções reais com Grok oficial são opt-in/separadas, com versão/plataforma registradas; não usar credenciais pessoais como fixture.


## Grok Build — decisões de revisão

- Ler `plans/grok-build/CODE_REVIEW_003.md` antes de reabrir decisões já corrigidas.
- A referência oficial atual adicionou `compat.codex.skills` e `compat.codex.hooks`: default explicitamente `false`.
- Remover env herdada de identidade e compatibilidade, mas preservar restrições administrativas `GROK_DISABLE_API_KEY_AUTH`, `GROK_FORCE_LOGIN_TEAM_ID`, sandbox e requirements.
- Config local do perfil pode optar deliberadamente por compat Claude/Cursor/Codex; não forçar `false` via env.
- Evidências de revisões antigas não descrevem disponibilidade atual de ferramentas.
  Consultar o estado vigente e distinguir launcher sintético de integração real.


## Store e View Limits

- O índice deve rejeitar aliases que apontem para o mesmo diretório físico; não relaxar `profileDirIdentity` para evitar data loss em `delete`. Arquivos JSON de metadados mantêm o limite de 8 MiB inclusive na leitura via `os.Root` e no recovery helper.
- A lista `grokClearEnv` remove apenas redirecionamentos/identidade herdados; preservar requisitos/guardrails de organização (sandbox, login team, disable API key). Revalidar contra a referência oficial por versão.
- `plans/view-limits/README.md` é **plano**, não adapter entregue. Só usar fontes oficiais autorizadas e operações read-only por perfil; diferenciar assinatura, API e uso local. Não extrair segredos ou fazer requisições de teste pagas inadvertidamente.
- Ler `plans/grok-build/CODE_REVIEW_004.md` para pendências nativas/ACL antes de promover suporte.

## Build local antes da entrega e do push

- Ao concluir alterações em uma CLI, gerar o executável nativo atualizado em
  `./dist/<nome>/<nome>` (com `.exe` no Windows) para o usuário testar.
- Antes de cada `git push`, confirmar que os binários das CLIs alteradas foram
  reconstruídos a partir do código atual e verificar help/versão em ambiente
  isolado. Usar Go 1.27.1 ou superior e as flags de versão da suíte quando a
  release estiver preparada; não alterar configurações globais.
- `dist/` contém builds locais regeneráveis e permanece ignorado pelo Git.
- Esta regra não reativa snapshots ZIP, Base64 ou contextos de entrega.
