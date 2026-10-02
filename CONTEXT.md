# Contexto atual — revisão do fluxo Go para v1.3.2

Data: 2026-10-01. Base inicial: v1.3.0 / 14dfc95. Integração: v1.3.1 / 5ba6009. Branch: codex/go-workflow-1.3.2.
Worktree: /Users/matheus/matheusvcouto/root/__worktrees/cli-tools/go-workflow.

## Escopo e integração

Regras, preparação de dependências, caches, geração de contratos e CI.
v1.3.1 / 5ba6009 integrada nesta worktree, preservando a UX, edição de texto,
cancelamento, cleanup, prefetch e notices do media-get. Contrato/produto
media-get 0.2.1 preservados. Usuário autorizou commit, push e publicação de
v1.3.2 após gates verdes; autorização recebida nesta conversa.
Release v1.3.1 publicada; workflow 36945709150 success, seis assets e
SHA256SUMS confirmados em https://github.com/matheusvcouto/cli-tools/releases/tag/v1.3.1. Não mover/reutilizar sua tag nem contornar seus gates.

Conflitos documentais resolvidos: prevalece preparação online e cache dedicado,
sem vendor como requisito. 381 arquivos de vendor comparados byte a byte com
módulos oficiais; vendor/modules.txt corresponde às versões pinadas. Os 382
arquivos foram movidos para dist/recovery/_vendor-1.3.1, não apagados. Lista em
dist/dependency-audit/vendor-paths.txt. Notices/licenças permanecem nos archives.

## Ambiente e regras vigentes

Go 1.27.1 disponível em /Users/matheus/.local/share/mise/installs/go/1.27.1/bin.
A instalação padrão em /usr/local/go tentou baixar outra toolchain; usar o
Go compatível já instalado no PATH, sem alterar configurações globais.
Preparação de módulos com rede é permitida. Checks reutilizam dist/go-cache
com downloads bloqueados e estado de aplicação isolado. Não gerar vendor por
limitação do ambiente nem snapshots ZIP/Base64/contextos separados de entrega.
Crashers de fuzz ficam na worktree persistente. Segurança e rollback mantidos.

## Evidência desta revisão

Preparação executada nesta base sem módulos externos. Regressão de módulos com
proxy sintético PASS: falha sem cache, preparação, duas execuções offline,
contratos com dependência preparada e manifests preservados.
scripts/check-safe.sh all PASS (fmt/test/vet/shuffle/race/API/contratos/changes).
scripts/check-safe.sh fuzz PASS nos oito alvos; nenhum crasher nesta rodada.
Actionlint oficial 1.7.12 darwin/arm64, SHA-256 upstream verificado: PASS.
Sintaxe shell e git diff --check PASS. Evidência local inicial; a execução remota da integração está registrada abaixo.
Revisão adicional: CI verifica integridade dos módulos e rejeita alterações
ou criação de go.mod/go.sum durante preparação. Testes de tools/release PASS,
actionlint PASS e git diff --check PASS após o ajuste. Guard do workflow
executado em Git sintético local: aceita estado limpo e rejeita manifest
alterado/go.sum novo. A execução nativa remota está registrada abaixo.
Regra reforçada: manter github.com/matheusvcouto/cli-tools sem /vN. Mudança de
major/path exige pedido específico do usuário, nunca inferência de release ou
plano antigo. go.mod e imports preservados; ajuste apenas documental.
Revisão completa e comandos: docs/go-workflow.md.

O contexto anterior está preservado em docs/history/go-workflow/context-before-1.3.2.md
como histórico, sem valor normativo para snapshots ou disponibilidade atual.

## Validação da integração

Preparação real de x/sys v0.48.0 e x/term v0.46.0 via proxy oficial e checksum
database PASS; go mod verify PASS. Gates da árvore integrada PASS: fmt/test/vet/shuffle/race/API/contratos/changes.
Actionlint PASS; builds nativos das três CLIs e smokes isolados PASS.
Builds reconstruídos a partir da árvore integrada antes do push.
Depois observar CI nativo, preparar v1.3.2, repetir CI e enviar tag/publicação.
Histórico do agente anterior preservado em
 docs/history/go-workflow/context-v1.3.1.md; não restaura regras de snapshots.

## CI da integração e correções

Run 36946751109: Linux amd64/arm64, macOS arm64 e Windows arm64 PASS.
macOS Intel falhou na limpeza de telemetria do Go; Windows amd64 falhou na
leitura prematura de PID sintético. Correções: semear o modo off na configuração
isolada antes de invocar Go (GOTELEMETRY não é variável configurável), e publicar
o PID completo por rename no teste Windows. Correções confirmadas em todos os jobs nativos do run 36947625672 (success).
Mise v1.3.1 PASS em HOME/MISE_*/GH_CONFIG_DIR sintéticos, sem configuração global.
Referências de API pública e conteúdo dos archives corrigidas em docs/cli-api.md
e docs/security.md. Nenhum comportamento de produto foi modificado.

Árvore corrigida: check-safe.sh all PASS e builds/smokes das três CLIs PASS.
CI nativo completo do commit 93c4e97 PASS (run 36947625672). Liberado preparo v1.3.2; commit preparado ainda deve passar CI antes da tag.

## Release preparada

v1.3.2 materializada; records arquivados em changes/archive/1.3.2.
Data do changelog em UTC, consistente com v1.3.1: 2026-10-02.
Árvore preparada: check-safe.sh all PASS, preflight v1.3.2 PASS,
três binários reconstruídos com SuiteVersion=v1.3.2 e smokes isolados PASS.
CI do commit preparado e workflow de publicação ainda pendentes.
Depois da publicação, iniciar a revisão de estimativas e velocidade do media-get
pedida pelo usuário; não instalar os aplicativos citados como referência.

## Publicação concluída e próxima revisão

Release v1.3.2 pública: https://github.com/matheusvcouto/cli-tools/releases/tag/v1.3.2.
Commit 9cf9f97: CI 36948413603 success; publicação 36948751821 success.
Seis archives + SHA256SUMS; smokes nativos dos archives PASS em toda a matriz.
Mise v1.3.2 PASS para as três CLIs em ambiente sintético, sem ativação pessoal.
Branch de revisão seguinte: codex/media-get-throughput, nesta mesma worktree.
Escopo autorizado: auditar estimativas, partes críticas, duplicação e velocidade.
Não mover v1.3.2 nem publicar outra versão sem autorização específica.

## Revisão local de media-get concluída

Default de fragmentos 4; override 1..8 interativo/flag, opção única no argv.
Tamanho exato tem prioridade; HLS desconhecido continua aproximado/variável.
Construção de previsões e resumo de configurações sem duplicação entre fluxos.
Regressões sintéticas e check-safe.sh all PASS; contrato atualizado, API pública
preservada, git diff --check PASS. Seis cross-builds PASS; smoke nativo PASS.
Binário: dist/media-get/media-get, suite_version=v1.3.2+dev. Produto fica 0.2.1
até prepare futuro, sem bump manual. Relatório: docs/media-get-throughput.md.
Nenhum aplicativo de referência instalado/executado, mídia real baixada ou conta
acessada. Throughput real e CI remoto desta branch não executados. Melhorias
locais ainda sem commit/push/release; v1.3.2 continua apontando para 9cf9f97.

## Build local para teste do usuário — 2026-10-02

Pedido atual: aceitar 25 fragmentos/flags, melhorar resumo e concluir antes de
criar release. Teto 64, default 4; 65 rejeitado antes de consultas. Resumo final
antes da confirmação e transferência, também em --yes; --name explícito não
repete pergunta. Stderr com quadro responsivo/texto redirecionado, controles
sanitizados e Referer/URL ocultos; erro de escrita impede baixar.

Validação desta árvore: check-safe.sh all PASS, git diff --check PASS,
seis cross-builds PASS e smoke nativo das três CLIs PASS. Logs em
 dist/release-validation/media-summary-{all,native,cross}.log.
Regressões: 25/64/65, flags/resumo antes da confirmação, privacidade, largura e
falha de saída. Primeiro teste apontou contrato desatualizado e caso antigo
usando 9 como inválido; corrigidos, gates finais verdes. Invocação inicial pelo
shim mise bloqueou por configuração não confiada; usar Go 1.27.1 direto no PATH,
sem tocar trust/configuração pessoal.

Executável atual: dist/media-get/media-get, suite_version=v1.3.2+dev, produto
0.2.1 até futuro release prepare. Sem commit/push/tag/release das melhorias.
CI remoto e medição real de throughput não executados; aguardar teste do usuário.

## Env e teto 256 — validação local 2026-10-02

CLI_TOOLS_MEDIA_GET_CONCURRENT_FRAGMENTS implementado com provider canônico;
flag > env > default 4. Faixa atual 1..256. Regressões de precedência, env vazio/
inválido, endpoints estáticos e repasse de 128/256 PASS. Gates completos locais
check-safe.sh all PASS (fmt/test/vet/shuffle/race/API/contratos/changes),
git diff --check PASS; build nativo e help/versões das três CLIs PASS. Logs
dist/release-validation/media-env-{all,native}.log. Executável atualizado em
dist/media-get/media-get, suite v1.3.2+dev. README específico atualizado.
Nenhuma configuração global ou mídia real foi tocada pelo agente; sem publicação.
Cross-build específico deste ajuste e CI remoto: não executados.

## Revisão final editável — 2026-10-02

Menu Baixar / Editar opções / Cancelar substitui s/N. Edições locais de tipo/
qualidade/legenda, nome, destino, Referer e fragmentos; flags/env substituíveis
nessa execução. Resumo reexibido, destino validado e cache preservado salvo
mudança de fonte. Regressões sintéticas de múltiplas edições, destino inválido,
resumo/pedido final, voltar e cancelamento PASS. check-safe.sh all PASS (fmt/
test/vet/shuffle/race/API/contratos/changes), git diff --check PASS. Build nativo
e smokes das três CLIs PASS. Logs media-review-{all,native}.log em
dist/release-validation. Executável dist/media-get/media-get reconstruído com
suite_version=v1.3.2+dev; sem release, CI remoto ou nova medição de throughput.

## Correção de publicação autorizada — v1.3.3 (2026-10-02)

O usuário autorizou retirar a publicação numerada incorretamente e corrigir para
v1.3.3, sem preservar sua release/tag. A release equivocada e suas tags local e
remota foram removidas. Alterações de produção permanecem as mesmas já testadas:
media-get 0.3.0 experimental, demais produtos 1.1.0, módulo canônico sem sufixo.
Changelog e arquivo de change records corrigidos para changes/archive/1.3.3.
Não renomear notas/resultados antigos como se tivessem aprovado outra versão.
Gates da árvore corrigida, CI remoto, tag e publicação ainda pendentes.

## Decisão do usuário — aprovação explícita da versão (2026-10-02)

Quando o cálculo/classificação sugerir uma versão diferente da sequência
esperada ou pedida pelo usuário (inclusive minor/major da suíte em vez do patch
esperado), PARAR antes de prepare --write, commit de preparo, tag ou publicação e
PERGUNTAR explicitamente. Apresentar resumo concreto das mudanças, versão atual,
versão esperada/proposta da suíte, versões individuais dos produtos e motivo da
divergência, incluindo eventual limitação do tooling. Aguardar aprovação explícita
da versão proposta ou escolha da alternativa. Aviso em commentary, preview ou
pedido genérico de publicar não equivalem a aprovação da divergência.

A correção atual é autorização específica de v1.3.3; não autoriza outros saltos.
O tooling ainda agrega o maior impacto dos componentes na suíte; antes de outro
preparo, revisar essa política com o usuário em vez de seguir o cálculo sozinho.
Não enfraquecer gates para forçar versões nem alterar path do módulo.

A retirada no GitHub não comprova ausência de caches externos do módulo Go.
Downloads registrados corresponderam aos smokes internos; isso não comprova
que nenhuma cópia externa existe. Não consultar uma versão retirada em proxies
para testar sua existência, pois a própria consulta pode provocar seu cache.
Reutilizar no futuro uma versão publicada e retirada exige discutir esse risco
com o usuário; nunca garantir ausência de conflitos de checksum sem evidência.

Árvore corrigida para v1.3.3: check-safe.sh all, preflight e smokes nativos
PASS. Logs em dist/release-validation/1.3.3/{all,native}.log. Código e manifests
de produto preservados; binários reconstruídos com suite_version=v1.3.3.
CI remoto, tag e publicação da correção ainda pendentes nesta etapa.

## Correção de formatos — v1.3.4 (pedido explícito)

Usuário autorizou implementação/testes/commit/push/publicação da suíte v1.3.4.
Corrigir escolha ausente SRT/TXT e oferecer MP4 compatível H.264/AAC; extensões
explícitas selecionam formato. Produto previsto 0.3.1 experimental, demais
produtos preservados; API pública do módulo sem alterações. Nenhuma mídia/conta
real é fixture. Gates e publicação pendentes; não declarar runtime real sem teste.

Gates locais da correção PASS: fmt/test/vet/shuffle/race/API pública/contracts/
change records; preview calcula exatamente suite 1.3.4 e media-get 0.3.1.
FFmpeg real local com mídia inteiramente sintética: remux H.264/AAC e conversão
VP9/Opus PASS, com decodificação de vídeo/áudio final. Testes negativos de
codecs, ausência de vídeo, perda de áudio, cancelamento e TXT inválido PASS.
Binários nativos reconstruídos e smokes isolados PASS (suite 1.3.3+dev).
Evidências em dist/release-validation/1.3.4; CI remoto ainda pendente.

CI nativo 37055188631 do commit 85c1927: success em todos os 12 jobs,
incluindo seis runners de testes. Preparo v1.3.4 materializado pelo tooling,
media-get 0.3.1 experimental e demais produtos preservados. Próximo gate:
checks/builds locais e CI do commit preparado antes de criar a tag.

Árvore preparada: check-safe.sh all, preflight v1.3.4 e builds/smokes
nativos das três CLIs PASS. Binários locais com suite_version=v1.3.4;
media-get --version=0.3.1. CI do commit preparado, tag e release pendentes.

## Publicação v1.3.4 concluída — 2026-10-02

Release Latest: https://github.com/matheusvcouto/cli-tools/releases/tag/v1.3.4.
Tag aponta para 73907e8, commit preparado aprovado pelo CI 37055942843.
Workflow release 37056511916 success; seis archives e SHA256SUMS publicados,
com smokes dos mesmos bytes em seis runners. Instalação pública via mise PASS
em HOME/MISE_*/GH_CONFIG_DIR sintéticos para as três CLIs, sem ativação global.
Produto media-get 0.3.1 experimental; ai-profile/repo-zip 1.1.0 preservados.
TXT/SRT, MP4 compatível e extensões explícitas documentados no README específico.
FFmpeg real com mídia sintética verificou imagem/áudio; o arquivo real relatado
pelo usuário não foi inspecionado, convertido nem usado como fixture. Não
declarar reprodução em todo player ou download Windows: capability preservada.
Receipt e logs: dist/release-validation/1.3.4/publication-receipt.json.
Binários nativos atualizados em dist/<tool>/<tool>; main contém a implementação.
