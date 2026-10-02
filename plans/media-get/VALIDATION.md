# Media Get — validação da rodada 2026-10-01

Ambiente observado: macOS ARM64; Go 1.27.1 instalado em
`/Users/matheus/.local/share/mise/installs/go/1.27.1/bin`. Baseline e módulo
permaneceram iguais. Nenhum pacote foi instalado, nenhum download real foi
executado e o projeto Deno original foi consultado somente por leitura.

## Execuções

1. Testes focados: `PATH=<Go 1.27.1>/bin:$PATH ./scripts/check-safe.sh test
   ./internal/mediaget/... ./cmd/media-get` — PASS.
2. Primeira execução de `check-safe.sh all` — FAIL no teste
   `tools/release.TestRepositoryCommandsAreDiscovered`: expectativa antiga
   `[ai-profile repo-zip]`; descoberta real `[ai-profile media-get repo-zip]`.
   A função exata foi revisada e a expectativa foi atualizada. Não houve
   alteração no mecanismo de descoberta ou relaxamento do gate.
3. Execução final: `PATH=<Go 1.27.1>/bin:$PATH ./scripts/check-safe.sh all` — PASS,
   exit 0. Executa fmt, `go test ./...`, `go vet ./...`, shuffle/count=3,
   `go test -race ./...`, `release api check`, `release contracts check` e
   `release changes validate` em HOME/cache/TMP/Git sintéticos e offline.
4. Builds CGO_ENABLED=0/trimpath/buildvcs=false com ambiente explícito,
   GOTOOLCHAIN=local/GOPROXY=off: darwin/arm64, darwin/amd64, linux/amd64,
   linux/arm64, windows/amd64 e windows/arm64 — todos PASS (compilação).
5. Binário darwin/arm64: `--version`, `--help`, `version --json` em ambiente
   sintético — PASS. Versão individual observada: `media-get 0.1.0`.

Binário local: `dist/media-get/media-get`. Builds das outras plataformas:
`dist/media-get-check/cross/`. São artifacts locais, não uma release publicada.

## Cobertura nova

- Precedência de destino, variável vazia, default HOME sintético/Downloads.
- Interação, Referer vazio/definido, voltar, cancelar/declinar e automação sem TTY.
- Metadata/estimativa sem tamanho, soma incompleta, bitrate, overflow e infinito.
- SRT manual/automático e escaping do identificador de idioma.
- Falha de dependência antes do spawn/escrita; causas e dicas de instalação.
- Argv de consulta/download, ausência de config/plugins/componentes remotos.
- Ambiente sem tokens/HOME/PYTHONPATH herdados; saída bruta do processo descartada.
- Timeout/cancelamento Unix com morte do grupo/descendente sintético.
- Saída vazia, symlink, incompleta, múltipla; identidade da área alterada.
- Colisão com arquivo/symlink, concorrência, não sobrescrita e recuperação parcial.
- Endpoints estáticos sem HOME/backend e lock de contrato byte a byte.

## Revisão de documentação oficial

O README e parser oficiais do yt-dlp confirmaram templates/progress-delta,
selectors, merge `mp4/mkv`, sub-langs por regex e conversão SRT. A revisão
retirou a flag inexistente `--no-netrc`; `usenetrc` é opt-in/default false no
parser, com config ignorada e sem argumentos de autenticação. Componentes
remotos são explicitamente desativados. Essa é evidência de revisão, não runtime.

- https://github.com/yt-dlp/yt-dlp/blob/master/README.md
- https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/options.py
- https://formulae.brew.sh/formula/yt-dlp
- https://formulae.brew.sh/formula/ffmpeg

## Não executado / limites

- yt-dlp/FFmpeg reais, rede, reprodução, vídeo do exemplo e contas reais.
- Runner nativo Linux ou Windows. Cross-build não comprova runtime.
- Fuzz adicional desta rodada (seeds da suíte executaram no teste normal).
- Instalação global, commit, push, preparação de release e publicação.
- Snapshot/checksums: verificados em `snapshots/009/` (CRC, entradas completas,
  contexto idêntico e Base64/SHA-256).

O comando novo é experimental. Sucesso com fakes demonstra o launcher e as
invariantes locais; não prova compatibilidade com um site/extrator upstream.

## Atualização — prévias paralelas e progresso

Go 1.27.1/macOS ARM64. `scripts/check-safe.sh all` PASS: fmt, suíte completa,
vet, shuffle/count=3, race, API pública, contratos e change records.

Regressões novas: estimativas aparecem antes dos prompts de seleção; valores
por selector preservam soma de vídeo/áudio; voltar reutiliza cache; queries
sobrepõem execução com limite de três workers; falha/timeout e parcela
incompleta mostram tamanho indisponível; cancelamento encerra workers sem
iniciar download ou preencher cache; animação aparece enquanto consulta está
bloqueada; progresso expõe bytes/percentual/speed/ETA e etapas; marcador com
texto extra é rejeitado; ETA extremo/NaN são descartados; exec.ExitError é
recuperável via errors.As e saída sensível continua oculta.

O primeiro teste focado da atualização falhou em
TestRefererAndIsolationAppliedToBothCalls porque a contagem antiga considerava
somente evento numérico. A assertion agora valida os três eventos de etapa e
transferência da fixture, inclusive processamento. A suíte completa posterior
passou. Ferramentas/sites reais e jobs nativos remotos continuam não executados.
Sem novos snapshots, commit, push ou release nesta etapa.

Após os ajustes finais de apresentação e preservação da classe de erro da CLI:
`check-safe.sh race ./internal/mediaget/...` PASS. Binário macOS ARM64 recompilado
com CGO_ENABLED=0/trimpath/buildvcs=false e ambiente sintético/offline; smokes
--help, --version e version --json PASS. `git diff --check` PASS.

## CI nativo antes do preparo de v1.3.0

Commit `805e6999036fa11230a6a23a1026719123e431fd`: CI
https://github.com/matheusvcouto/cli-tools/actions/runs/36933364310 — completed/success.
Todos os 12 jobs passaram: testes em Linux/macOS/Windows × x64/ARM64,
workflow-lint, change records, contratos, API pública, shells nativos e
cross-build. Isso comprova runtime das capabilities exercitadas pelos testes
sintéticos; download de media-get no Windows continua unsupported.

Usuário forneceu execução real Twitch com metadados, estimativas e transferência
em andamento. Não foi observada conclusão/reprodução; esta evidência do usuário
não foi usada como fixture. Suite v1.3.0 escolhida expressamente pelo usuário;
prepare --write executado após CI verde, preservando versões independentes.
CI no commit preparado e workflow de release ainda devem passar antes de
declarar publicação concluída. Sem novos snapshots.

Após prepare --write: gates completos locais novamente PASS; preflight v1.3.0
PASS (três ferramentas, nenhum record pendente). Binário em dist/media-get
reconstruído com versão de produto 0.2.0/suíte 1.3.0; smokes help/version/json
PASS. Arquivos de preparação e records arquivados conferidos por leitura.

## Experiência interativa — implementação local, aguardando aprovação

2026-10-01, Go 1.27.1, macOS ARM64. Interface com setas/Enter/busca/Esc;
Referer removido da pergunta inicial e disponível em catálogo opcional na revisão
ou após falha de consulta. Trocar Referer renova metadados/cache; fragmentos
paralelos (1..8, default 1) afetam só a transferência e não repetem consultas.
Barra atualiza a mesma linha com bytes/total/speed/ETA; aproximação e total
indisponível explícitos; conversão/publicação animadas, sem percentual inventado.
Cancelamento das consultas aguarda cleanup/reap do backend antes de encerrar.

Evidências executadas:

- `check-safe.sh all`: PASS (fmt, testes, vet, shuffle ×3, race, API pública,
  contratos e validação do change record). Ajuste final de largura do spinner:
  testes focados de `internal/mediaget/cli` novamente PASS.
- Testes cobrem recuperação da consulta por Referer, renovação de cache,
  fragmentos inválidos, não repetição de escolhas, barra/unknown/estimated,
  largura estreita, sanitização, polling cancelável sem leitor residual,
  cleanup de backend antes de retornar e cancelamento em falha de output.
- Cross-build CGO_ENABLED=0: darwin/linux/windows × amd64/arm64 PASS;
  isso não comprova UI nativa nos demais sistemas.
- Binário atual `dist/media-get/media-get`, reconstruído após última alteração;
  smokes isolados/offline --help/--version/version --json PASS.
- Pseudo-terminal nativo macOS: setas, busca por configuração Referer, reconsulta,
  download sintético com barra, Ctrl+C e SIGTERM PASS. Flags de modo/echo/sinais
  restauradas (PENDIN do driver macOS excluído da comparação de bookkeeping).
  Probe reproduzível em `dist/ux-check/terminal-smoke.py`; fixtures/saídas locais
  sintéticas, nenhuma URL real, conta, cookie ou ferramenta multimídia real.
- go.sum e vendor fixam x/term v0.46.0 / x/sys v0.48.0; licenças BSD comparadas
  byte a byte. Notices completos incluídos pela ferramenta de release e testados.
- `git diff --check`: PASS. `dist/` e snapshots continuam ignorados; nenhum
  snapshot está rastreado. Não foram gerados novos snapshots.

Não executados nesta revisão: yt-dlp/FFmpeg reais, playback, benchmark de rede,
novo CI remoto, release prepare/write, commit, push, tag ou publicação. A UI
Linux requer validação nativa; download Windows continua indisponível.
Versões publicadas permanecem intactas; v1.3.1 aguarda aprovação do usuário.
O ganho de fragmentos depende do protocolo/servidor e não foi medido.

## Revisão do relato de barra ausente/total desconhecido

2026-10-01, macOS ARM64/Go 1.27.1. Bugs confirmados por revisão e regressões:
ParseInt descartava total_bytes_estimate HLS fracionário e ETA fracionário; o
wizard descartava a previsão antes de criar renderer; encerramento apagava a
última linha da barra. Corrigidos com parser numérico finito/bounded, previsão
passada ao renderer, barra imediata com spinner inicial e preservação da linha.

Template acrescenta format_id JSON convertido para identidade SHA-256 opaca;
previsões e eventos podem contabilizar globalmente streams sem duplicar bytes.
Totais aproximados marcados; só dados reais refinam parcelas, nunca inferir troca
por diminuição dos bytes. Sem previsão global, progresso é por stream. Testes
incluem decimal/notação científica/overflow/NaN, ETA, preview conhecido sem dados,
streams vídeo/áudio e eventos repetidos, fallback de total, linha em cancelamento.

Interface compacta cada menu concluído em uma resposta. Quando alturas de vídeo
são integralmente conhecidas, omite caps iguais/acima do máximo (melhor qualidade
já cobre todos), exibe máximo informado da fonte e evita queries redundantes.
Formatos sem codec de vídeo não poluem alturas. Valores das flags são preservados.
Check áudio agora confere ffprobe da localização efetiva do ffmpeg, rejeitando
probe explícito incoerente por os.Stat/SameFile antes de executar o filho.

Evidência final:

- `check-safe.sh all`: PASS após a última alteração de produção: fmt, testes,
  vet, shuffle ×3, race, API/contratos e change record. Record final revalidado.
- Build nativo atualizado, smokes isolados --help/--version/version --json PASS;
  darwin/linux/windows × amd64/arm64 cross-build com CGO_ENABLED=0 PASS.
- Pseudo-terminal macOS com fakes que emitem HLS decimal e identidades de formato:
  áudio, vídeo+áudio, busca/configuração e menus compactos PASS; estimativa não
  vira total desconhecido. Ctrl+C no selector e SIGTERM restauram terminal: PASS.
- Teste separado com PTY controlando a sessão/foreground do filho: enviar byte
  Ctrl+C durante transferência gera SIGINT real, retorna 130 e preserva a última
  barra/área incompleta: PASS. O foreground é validado no filho antes de exec;
  master é drenado até a saída para não bloquear teardown de PTY no macOS.
  A restauração de flags é medida separadamente nos casos sem controlling session,
  pois macOS desassocia esse PTY ao encerrar o session leader (ENOTTY no parent).
- Reprodutor sintético atualizado em dist/ux-check/terminal-smoke.py, dentro do
  repositório e ignorado; nenhuma URL, título, conta ou mídia real copiada para
  fixtures. `git diff --check` PASS. Nenhum snapshot gerado/rastreado.

A consulta às fontes oficiais confirma que fragment.py calcula estimativas por
float e common.py expõe info/progress ao template; isso é revisão, não prova de
compatibilidade com a versão instalada pelo usuário. Twitch/FFmpeg reais e novo
CI remoto não executados nesta revisão. Não houve encerramento espontâneo nos
probes. O log fornecido contém context canceled/130, mas não identifica quem
cancelou: usuário confirmou posteriormente ter pressionado Ctrl+C; portanto o 130 desta
execução é esperado, sem evidência de encerramento espontâneo.
Nenhuma mudança nos arquivos incompletos pessoais; nenhum commit/push/tag/release.

## Descarte padrão de incompletos — 2026-10-01 (não publicado)

Solicitação: descartar por padrão após falha/cancelamento, com opção explícita
para manter e pasta visível. Implementação em domínio/CLI, sem novas flags,
dependências ou mudanças na API pública do core.

- `scripts/check-safe.sh all`: PASS (fmt/test/vet/shuffle/race/API/contratos/changes).
- Regressões: falha descarta somente área atual e preserva pastas antigas;
  retenção explícita, tamanho e descarte; substituição de identidade bloqueada;
  decisão após contexto cancelado, manter, descarte padrão e erro de interação.
- Terminal sintético macOS: downloads áudio/vídeo, cancelamento de selector,
  SIGTERM, SIGINT real durante transferência escolhendo descartar e manter:
  seis casos PASS. Pós-SIGINT retorna 130 nas duas decisões.
- Cross-build CGO=0 nos seis alvos e smokes estáticos nativos: PASS.
- `dist/media-get/media-get` reconstruído a partir da fonte atual.

Pastas pessoais, incluindo o incompleto mencionado pelo usuário, não foram
alteradas. Testes usam somente backends/ferramentas sintéticas. Nenhum commit,
push, tag, release ou snapshot. SIGKILL/crash/desligamento pode deixar área
visível: não existe execução de cleanup após encerramento forçado.

## Prefetch sem bloquear menus — 2026-10-01 (não publicado)

- Vídeo/áudio começam junto da metadata inicial; qualidades aplicáveis entram
  na fila antes de escolher vídeo. Três workers, prazo de 20 s por fonte,
  fila limitada e cache sincronizado somente em memória.
- Regressões com backend bloqueado provam consulta antecipada dos dois tipos,
  qualidade antes da escolha, menu selecionável durante consultas pendentes,
  ausência de duplicação ao reenfileirar/voltar, limite de concorrência e join
  de workers no cancelamento. Referer renova cache; fragmentos o conservam.
- `scripts/check-safe.sh all`: PASS (fmt/test/vet/shuffle/race/API/contratos/changes).
- Terminal sintético com estimates atrasadas em 0.8 s: PASS nos seis casos.
  Verificado menu com calculando, seta para áudio enquanto pendente, tamanho
  surgindo no mesmo menu antes de Enter e continuidade da opção selecionada.
  Downloads áudio/vídeo, restauração, SIGINT/SIGTERM e descarte/manter passaram.
- Cross-build nos seis alvos CGO=0 e smokes help/version/JSON: PASS.
- Binário atualizado em dist/media-get/media-get, sem nova dependência.

Apenas ferramentas sintéticas: não é benchmark de Twitch/yt-dlp reais. Cache
não é gravado em disco. Antes do download, consultas são canceladas/recolhidas
sem aguardar tamanho; somente a previsão já pronta segue para o renderer.
Nenhuma pasta pessoal foi alterada, nenhum push/tag/release/snapshot gerado.

## Edição nativa de texto — 2026-10-01 (não publicado)

- Campos TTY Unix agora editam com esquerda/direita, Home/End, Ctrl+A/Ctrl+E,
  Delete/Backspace, runes Unicode e viewport horizontal. Fallback preservado.
- CSI completos consumidos, incluindo modificadores/marcadores de paste;
  comandos de cursor não viram texto de URL/nome. Nenhuma dependência nova.
- Entrada nativa síncrona garante term.Restore antes de sair por cancelamento.
- Mensagem de Ctrl+C mostra Cancelado e outcome de cleanup, mantendo Unwrap/130.
- Testes de modelo/decoder/cancelamento: PASS. PTY sintético macOS: oito casos
  PASS, incluindo edição e arquivo gerado bç!, ausência de escapes literais,
  Ctrl+C e SIGTERM no campo nome com termios restaurado; demais menus/progresso
  e retenção/descarte passaram. Nenhuma mídia/conta pessoal usada.
- Shuffle identificou teste de prefetch que assumia ordem entre workers:
  corrigido para aguardar readiness de todas as entradas verificadas.
- Gates completos finais (fmt/test/vet/shuffle/race/API/contratos/changes): PASS.
- Cross-build nos seis alvos e smokes nativos: PASS; binário reconstruído em
  dist/media-get/media-get. Sem push/publicação/snapshots.

## Publicação v1.3.1 autorizada — preparo

- Implementação enviada: 80a20415a560647d20c5ed778f3562ebcf37fbf7.
- CI real 36944693037: success em todos os jobs, incluindo seis runners nativos,
  race, contratos, API pública, workflows, completions e cross-build.
- Preview e prepare --write: suíte v1.3.1, media-get 0.2.1 experimental; demais
  produtos 1.1.0. Records consumidos preservados em changes/archive/1.3.1.
- CI do commit preparado e workflow de release ainda pendentes nesta etapa.

## Revisão local de throughput após v1.3.2

Base publicada 9cf9f97; branch codex/media-get-throughput. Gates locais completos
PASS, incluindo regressões de total exato/HLS variável e default/override/faixa
antes de consultas. API pública e contratos PASS. Seis cross-builds PASS; build
nativo/help/versões isolados PASS com SuiteVersion=v1.3.2+dev. Windows download
continua indisponível; cross-build não comprova runtime. Nenhuma execução real
de Twitch/YouTube/FFmpeg ou medição de rede; CI remoto dessa branch não executado.
Relatório e fontes pinadas: ../../docs/media-get-throughput.md. Não publicada.

## Resumo e paralelismo ampliado — 2026-10-02

Gates completos locais PASS nesta árvore: fmt/test/vet/shuffle/race/API/contratos/
changes. Seis cross-builds PASS e smokes nativos PASS. Regressões sintéticas
cobrem 25/64 válidos, 65 rejeitado antes de consultar, flags sem repetir nome,
resumo antes da confirmação, largura, privacidade e falha de escrita que impede
transferência. Executável dist/media-get/media-get, suite v1.3.2+dev. Logs:
 dist/release-validation/media-summary-{all,native,cross}.log.
Sem CI remoto, benchmark real, push ou nova release. Teste de rede pelo usuário
continua pendente; não inferir ganho de throughput dos testes sintéticos.

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

## Correção autorizada da versão da suíte — v1.3.3 (2026-10-02)

Release/tag com numeração não aprovada retiradas por pedido explícito do usuário.
Árvore corrigida preserva media-get 0.3.0 e as melhorias validadas pelo usuário,
com suíte v1.3.3. Changelog/arquivo de records ajustados; não reaproveitar resultados
de uma publicação anterior como evidência da versão corrigida. Gates locais,
CI, smokes de pacotes e mise isolado desta publicação ainda pendentes.

Árvore corrigida para v1.3.3: check-safe.sh all, preflight e smokes nativos
PASS. Logs em dist/release-validation/1.3.3/{all,native}.log. Código e manifests
de produto preservados; binários reconstruídos com suite_version=v1.3.3.
CI remoto, tag e publicação da correção ainda pendentes nesta etapa.
