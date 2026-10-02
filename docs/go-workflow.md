# Revisão do fluxo Go — destino v1.3.2

## Decisões

Base isolada: tag v1.3.0 / 14dfc95; branch codex/go-workflow-1.3.2.
O checkout original permanece com o trabalho independente para v1.3.1.
v1.3.1 / 5ba6009 foi integrada; gates da árvore combinada precedem o preparo de v1.3.2.

| Regra/comportamento anterior | Decisão e motivo |
| --- | --- |
| GOMODCACHE vazio por check | Cache dedicado persistente; módulos preparados são reutilizados. |
| GOCACHE vazio por check | Cache dedicado persistente; evita recompilar tudo em cada rodada. |
| Downloads bloqueados sem preparação definida | Preparação explícita com rede; checks continuam sem download implícito. |
| Gerador de contratos com cache vazio próprio | Reutiliza caches do Go invocador; mantém HOME sintético e downloads bloqueados. |
| Suposição permanente de módulo stdlib-only/sem go.sum | Removida; política existente já permite módulos externos justificados. |
| Vendoring para compensar ambiente limitado | Não é padrão; usar go.mod/go.sum e cache. Não apagar vendor de outra implementação. |
| Falta de Go/rede tratada como estado vigente | Verificar ambiente atual; Go 1.27.1 está instalado nesta máquina. |
| Snapshots ZIP/Base64 como entrega obrigatória | Removida das instruções ativas; artifacts antigos e evidências preservados. |
| Fuzz em cópia temporária descartada | Fuzz na worktree; crashers e cache sobrevivem à rodada. |
| Proposta antiga de /v2 e releases antigas em planos/docs | Explicitamente histórica; módulo e versões atuais continuam soberanos. |
| HOME/credenciais/Git/configuração de aplicação isolados | Mantidos: protegem estado real, independentemente do ambiente. |
| Rollback, containment, contratos, change records e gates nativos | Mantidos: garantias do produto, não limitações de sandbox. |
| GOTOOLCHAIN=local durante checks e Go mínimo 1.27.1 | Mantidos; uma toolchain compatível pode ser preparada previamente. |

Arquitetura do CLI Core, nomes de pacotes, stdlib-first e CGO de release são
escolhas explícitas do projeto. Não há base para classificá-las como regras
inválidas da comunidade Go. Nenhuma exige recriar bibliotecas maduras nem
impede módulos externos quando necessários. Instalações JS/TS não fazem parte
desta revisão; suas políticas globais não devem ser aplicadas como proibição
de módulos Go.

## Uso

Com Go 1.27.1 ou superior já selecionado no PATH:

```sh
./scripts/check-safe.sh prepare
./scripts/check-safe.sh all
./scripts/check-safe.sh fuzz
```

Preparação usa go mod download all e go mod verify. Dependências novas ou
mudanças de versão usam go get pacote@versão e go mod tidy quando necessário,
com revisão do diff e justificativa conforme ADR. Os checks usam -mod=readonly
para recusar atualizações necessárias de go.mod; preparar go.sum antes dos
checks. A flag não garante imutabilidade geral byte a byte de go.sum, por isso
CI também verifica o estado dos manifests após a preparação.
Caches ficam em dist/go-cache/{mod,build,path}; estado descartável dos checks
fica em dist/go-cache/runs. dist já é ignorado pelo Git. Não há limpeza dos
caches persistentes por execução nem dependência de vendor. O runner não lê
GOENV pessoal, GOWORK ou configuração Git global/sistema.

GOPROXY=off e GOVCS=*:off impedem downloads de módulos pelo Go. Eles não
implementam firewall nem bloqueiam toda a rede de um processo de teste.
Preparação pública mantém a checksum database habilitada por padrão. Um proxy
local sintético de teste pode explicitamente usar GOSUMDB=off, pois seu módulo
não existe na database pública; isso não é recomendação para pacotes públicos.

Referências primárias: [módulos e preparação](https://go.dev/ref/mod#go-mod-download),
[cache de módulos](https://go.dev/ref/mod#module-cache),
[autenticação](https://go.dev/ref/mod#authenticating),
[toolchains](https://go.dev/doc/toolchain).

## Validação observada

- Go 1.27.1 / macOS ARM64.
- Regressão com proxy local sintético: cache ausente falha com instrução de
  preparação; preparação preenche cache; duas execuções offline passam com
  proxy indisponível; contratos usam dependência preparada; go.mod/go.sum não
  mudam durante checks; HOME e secrets sintéticos não são herdados.
- scripts/check-safe.sh all: PASS (fmt/test/vet/shuffle/race/API/contratos/changes).
- Formatação e sintaxe shell após ajuste final: PASS.
- Actionlint oficial 1.7.12 darwin/arm64 com SHA-256 upstream verificado: PASS.
- scripts/check-safe.sh fuzz: PASS nos oito alvos, sem crashers nesta rodada.
- CI remoto, execução nativa Linux/Windows desta revisão e integração com
  v1.3.1: não executados. Download de módulos públicos externos nesta base
  stdlib-only: não necessário; regressão de módulos usa proxy sintético offline.

## Revisão adicional

As decisões de módulos/caches seguem os mecanismos do Go. A execução offline
é uma política deste projeto, não uma exigência universal do Go. Vendoring
também é suportado pelo Go e pode ser adequado em outros projetos; nesta suíte
não é necessário para compensar as limitações antigas do ambiente.

Melhoria aplicada ao CI: preparação executa go mod verify e rejeita mudanças
ou criação de go.mod/go.sum, incluindo arquivo não rastreado. Isso impede que
um commit com checksums incompletos seja corrigido silenciosamente durante CI.
A preparação local continua podendo completar go.sum para revisão do diff.
Após esse ajuste: testes de tools/release PASS, actionlint PASS e
git diff --check PASS. O guard extraído do workflow também passou em Git
sintético local: aceita manifests limpos, rejeita alteração rastreada e
rejeita go.sum novo não rastreado. Evidência sintética em dist/ci-guard-tests/.
CI remoto permanece não executado.

A identidade do módulo foi reforçada em AGENTS.md: manter exatamente
`github.com/matheusvcouto/cli-tools`; nenhuma migração para /v2, /v3 ou outro
/vN pode ser inferida de manutenção, releases ou documentos antigos. Uma quebra
inevitável da API pública requer decisão específica do usuário antes de mudar
major/path. ADR e engenharia foram alinhados; go.mod e imports não mudaram.

## Integração com v1.3.1 — aplicada

v1.3.1 / 5ba6009 integrada sem alterar a nova UX/cancelamento/prefetch do produto.
Docs de dependências atualizadas para cache preparado. 381 arquivos de vendor
são idênticos aos módulos oficiais; os 382 arquivos, incluindo modules.txt,
foram movidos para dist/recovery/_vendor-1.3.1. Lista prévia em
 dist/dependency-audit/vendor-paths.txt. Notices preservados na distribuição.
Módulos oficiais baixados com checksum database e go mod verify PASS.
Gates integrados PASS, actionlint PASS, três builds/smokes nativos PASS.
CI remoto da integração pendente antes de preparar v1.3.2.
v1.3.1 publicada com workflow 36945709150 success e todos os assets.
O change record cobre module:patch e documentação media-get:none, sem mudar
versões de produtos ou sua API. Usuário autorizou commit/push/publicação.

## Falhas encontradas no CI nativo

Run 36946751109 passou em Linux amd64/arm64, macOS arm64 e Windows arm64.
No macOS Intel, um subprocesso de telemetria disputou a limpeza da configuração
temporária: GOTELEMETRY é uma saída não configurável de go env. O runner agora
cria o modo off no diretório de configuração isolado antes de iniciar Go; a
regressão consulta o modo efetivo. No Windows amd64, o leitor observou o arquivo
de PID sintético vazio entre criação e escrita. O teste publica o arquivo
completo por rename; cancelamento de produção não mudou. A correção depende
de confirmação nos respectivos jobs nativos antes de preparar a release.
