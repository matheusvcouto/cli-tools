# CONTEXT — SNAPSHOT-008 / cli-tools

**Data:** 2026-09-26. **Repositório:** `cli-tools`; **módulo:** `github.com/matheusvcouto/cli-tools` (sem sufixo `/v2`); **próxima suíte:** `v1.1.0`, ainda não tagueada; **Go mínimo:** `1.27.1`. **Base:** SNAPSHOT-007, com a decisão de versão ajustada depois da importação. Relatório integral: `plans/release-readiness/SNAPSHOT_008_REVIEW.md`.

## Estado e escopo

- `ai-profile`: `claude`, `codex` e `grok` com `list/new/rename/delete/run/acp` e diretórios/ambientes de configuração separados. Grok ACP `grok agent <opções> stdio`, sem aprovações automáticas. `GROK_HOME` não impede a leitura de instruções do workspace nem representa sandbox absoluto.
- Armazenamento: `os.Root`, lock por SO, backup de índice, limites 8 MiB, identificação de órfãos, migração NUON explícita e delete com identidade integral sob lock (corrigido em 007). Windows: DACL protegida do root, LockFileEx, atomicidade confinada, Job Objects antes de liberar processo, e launch de npm conhecido por Node sem shell. Atenção: ACEs explícitas em arquivos Windows legados exigem auditoria nativa.
- GitHub Actions: CI com seis runners Linux/macOS/Windows amd64+arm64, tests/vet/race quando suportado, actionlint pinado/verificado e contratos. Release faz build único, atesta no builder, verifica SHA e executa seis smokes dos mesmos bytes antes de publicar draft conferido. **008** acrescenta `preflight` fail-closed antes da matriz e de shell completions: actionlint, changes validate, versão/changelog canônicos, path do módulo compatível com a tag, Go, presença de CLIs, ancestralidade do commit e tag remota. Scripts de tag alinharam regex canônico estável com builder Go. Testes para pending changes (inclusive symlink) adicionados.

## Critério de prontidão — diferenciação obrigatória

- **Pode enviar SNAPSHOT-008 como PR/branch para testar de verdade no GitHub, SEM TAG.** A análise estática é um sinal de preparo do código, **não prova de funcionamento geral nem de sucesso da CI**. Static 008: `gofmt` 119 Go, `bash -n` cinco scripts, JSON 15, TOML 1, PyYAML duas workflows/14 jobs, grafos `needs`/SHA sintaticamente coerentes; actionlint real não disponível aqui. Sem execução do Go 1.27.1 nesta máquina, os novos testes Go e vet não foram rodados. Tentativa de download oficial Go falhou por rede/DNS; não reduzir baseline.
- **NÃO distribuir/taguear como release estável ainda:** observar CI desta revisão e Windows x64/ARM64 real; executar instalações/autenticações reais Claude/Codex/Grok e ACP de cada um; checar disponibilidade real Grok ARM64, ACL explícita em perfis legados e migração a partir de backup. CI antiga não vale para código novo. Depois `release prepare --suite-version v1.1.0 --write`, commit preparado, reexecutar CI e `preflight` local; só então tag protegida, release smoke em seis targets e verificação da publicação remota.
- Change records pendentes atuais são **intencionais**: `preflight` deve rejeitar tag na árvore atual até o `prepare --write` consumir os records. Não remover gates para obter aprovação.

## Futuro — View Limits

- `plans/view-limits/README.md`: adaptadores read-only por provider/perfil (Codex, Claude Code, Grok), com consentimento, para limites de assinatura, cotas de API e uso local **separados**; nenhum token extraído pelo fluxo ordinário. A implementação não foi iniciada.

## Regras permanentes

- Nunca criar mocks/testes artificiais nem alegar que testes não rodados passaram. Compensar bloqueios externos com análise de código real e documentação oficial. Não alterar produção para adaptar ao sandbox, nem enfraquecer segurança ou gate. `fail-closed` sempre que apropriado; após corrigir um problema, prosseguir para o próximo possível.
- Entregar **repositório completo ZIP**, `CONTEXT.md` também separado e **Base64 puro** do ZIP, sem cabeçalho/fence. Nomes: `SNAPSHOT-{NNN}_cli-tools.zip`, `CONTEXT-{NNN}_cli-tools.md`, `SNAPSHOT-{NNN}_cli-tools_BASE64.md`; relatório e checksums adicionais. Validar integridade do ZIP, árvore extraída, SHA e reconstrução Base64. Preferir anexos ao fim, sem cards personalizados; caso a UI não renderize anexo nativo, usar os arquivos disponibilizados e o Base64 puro.
- macOS: `base64 -D -i ~/Downloads/SNAPSHOT-008_cli-tools_BASE64.md -o ~/Downloads/SNAPSHOT-008_cli-tools.zip`.

## Atualização 2026-10-01 — Media Get / SNAPSHOT-009

- Novo produto experimental `media-get` integrado ao CLI Core, com plano em
  `plans/media-get/README.md` e evidência atual em `plans/media-get/VALIDATION.md`.
- Referer opcional (interpretação do exemplo de “Repair”), vídeo com áudio,
  M4A e SRT; destino por flag/env/default Downloads, estimativa tolerante,
  progresso numérico, dependências do sistema e publicação sem clobber.
- Go 1.27.1 disponível nesta rodada: `scripts/check-safe.sh all` PASS, incluindo
  tests/vet/shuffle/race/API/contratos/changes. Cross-build do novo produto PASS
  nos seis alvos; smokes estáticos do binário macOS ARM64 PASS. Estas evidências
  locais atualizam a indisponibilidade de Go descrita no histórico acima, sem
  comprovar runtime de providers/sites ou jobs remotos.
- Nenhuma mídia/conta real testada. Linux nativo/Windows download não validados;
  download Windows explicitamente indisponível. Não houve commit/push/release,
  alteração de versões existentes ou de configurações reais.
- Entrega desta rodada: `snapshots/009/`; contexto separado, ZIP de toda a árvore
  de fontes e Base64 puro. ZIP exclui `.git`, caches/builds e artifacts ignorados;
  inclui arquivos versionados e novos arquivos de código/plano/documentação.

## Revisão 2026-10-01 — SNAPSHOT-010

Revisão de media-get sem alterar produção/contratos. Relatório:
plans/media-get/REVIEW_010.md. Testes focados herméticos PASS com Go 1.27.1.
Pendentes: descoberta de legendas sem formatos de vídeo; coerência entre ffprobe
checado e executável usado; causa estruturada de exec.ExitError. Achados de revisão,
sem reprodução com ferramentas reais. Validação real continua não executada.
Preferências tomadas pela IA e fluxo de uso foram documentados para avaliação
do usuário. Entrega em snapshots/010; nenhuma instalação/conta/commit/release.

## Atualização — prévias e progresso do media-get

Estimativas nos menus de tipo/qualidade antes da escolha, até três consultas
paralelas, timeout de 20 s por lote e cache local ao wizard. Carregamento com
animação/contador; transferência com bytes/percentual/speed/ETA e etapas de
processamento/publicação. Erros do yt-dlp agora preservam a causa estruturada
com mensagem segura. Gates completos locais PASS (Go 1.27.1/macOS ARM64).
Ferramentas reais, runners remotos e os demais achados da revisão continuam
pendentes; não houve push/publicação. Geração de snapshots desativada pelo usuário.

## Preparação do push / suíte v1.3.0

Usuário autorizou push e escolheu v1.3.0, inexistente no remoto consultado;
última release publicada v1.1.0. Preview com avanço explícito: suíte 1.3.0,
media-get 0.2.0 experimental; ai-profile/repo-zip permanecem 1.1.0.
Tooling ganhou --allow-suite-version-override com guards de avanço/major e
testes. AGENTS.md exige binário nativo em dist antes da entrega/push.
Gates locais completos novamente PASS; build local e smoke de versão PASS.
Primeiro enviar implementação e aguardar CI nativo; preparar release e
revalidar o commit antes de tag. Select com setas/barra visual ficou para
próxima etapa por pedido do usuário. Evidência Twitch fornecida pelo usuário
mostra consulta/estimativas e transferência iniciada; não comprova conclusão.

## CI verde e release preparada

CI 36933364310 do commit 805e699: todos os 12 jobs PASS, incluindo os seis
runners nativos. prepare --write materializou suíte 1.3.0 e media-get 0.2.0
experimental; change records movidos integralmente para changes/archive/1.3.0.
Falta confirmar CI do commit preparado, enviar tag nova e acompanhar smokes
e publicação. Evidência real Twitch enviada pelo usuário cobre início de
transferência, não arquivo final. Interface visual ficará para próxima etapa.

## Experiência media-get — local, aguardando aprovação de v1.3.1

v1.3.0 foi publicada na rodada anterior. Nesta rodada: menus com setas/busca,
barra na mesma linha e totais exatos/aproximados/desconhecidos, animação durante
processamento, Referer opcional na revisão (também após erro inicial) e catálogo
fechado de Referer/fragmentos paralelos 1..8. Sem configs persistidas. Troca de
Referer renova metadata/cache; configurar fragmentos não repete consultas.
Dependências oficiais x/term/x/sys fixadas e vendorizadas para checks offline;
notices BSD acompanham assets pela ferramenta de release.

Gates completos locais PASS, mais testes focados após ajuste final do spinner.
Cross-build seis alvos PASS; menus/barra/cancelamento/restauração testados em
pseudo-terminal nativo macOS com backend sintético. Binário atualizado em
`dist/media-get/media-get` e smokes isolados PASS. Evidência/limites detalhados em
`plans/media-get/VALIDATION.md`. Sites reais, benchmark e novo CI não executados.
Nenhum commit/push/tag/release prepare nesta rodada; aprovação de v1.3.1 pendente.
Snapshots automáticos continuam desativados; instruções históricas acima são
inativas conforme AGENTS.md. Nenhum snapshot rastreado ou gerado nesta rodada.

## Correção após relato de progresso — aguardando teste real/aprovação

Corrigidos bugs de totais/ETA HLS decimais, previsão perdida entre wizard e barra,
e limpeza da linha ao interromper. Barra começa com previsão e mantém dados na
saída; streams são contabilizados por ID opaco, sem duplicar bytes. Menus ficam
resumidos após seleção; caps redundantes de altura conhecida não exigem queries;
ffprobe validado na localização efetiva de FFmpeg por identidade física.

Gates completos, race, contratos, cross-build seis alvos, rebuild/smokes do binário
nativo e cinco cenários de pseudo-terminal macOS PASS. Incluem HLS decimal,
áudio/vídeo+áudio e SIGINT real durante download mantendo barra/área incompleta.
Detalhes/limites em plans/media-get/VALIDATION.md. Encerramento espontâneo não
reproduzido; usuário confirmou ter pressionado Ctrl+C: o cancelamento 130 foi esperado. Ferramentas/sites reais não executados, incompletos pessoais intactos.
Sem publicação/push/prepare; v1.3.1 ainda depende de aprovação. Sem snapshots.

## Descarte padrão após falha/cancelamento (não publicado)

Área privada agora visível (Media Get — Incompletos-<aleatório>). Interativo
oferece descartar (default) ou manter após o renderer/backend encerrar; novo
contexto de sinais e timeout de 30 s permite escolher depois do Ctrl+C. Esc,
novo Ctrl+C/EOF/timeout descartam; --yes descarta sem prompt. Limpeza confinada
revalida identidades e só remove a área desta execução. Caminho e tamanho são
informados; falha de limpeza preserva diagnóstico/caminho. Pastas antigas nunca
são varridas, e o incompleto pessoal citado permanece intocado.
Gates completos, seis cross-builds, seis probes de terminal sintéticos e smokes
nativos PASS; binário em dist/media-get/media-get atualizado. Sites reais não
executados. v1.3.1 continua dependendo da aprovação; sem push/release/snapshots.

## Prefetch e seleção sem espera (não publicado)

Vídeo/áudio estimados junto da metadata inicial; qualidades enfileiradas após
identificar formatos, antes de abrir seus menus. Três workers, 20 s por fonte,
cache sincronizado em memória até nome/confirmação. Menus nativos atualizam
spinner/tamanho por Poll de 150 ms e busca usa labels estáveis; fallback recebe
snapshot não bloqueante. Selecionar ou continuar nunca espera por estimates.
Referer cancela/recolhe sessão anterior; fragmentos mantêm cache. Antes do
download, encerrar workers e transferir só estimate pronta ao renderer.
Gates completos/race, seis cross-builds, smokes e seis probes de terminal com
queries lentas sintéticas PASS. Binário local atualizado. Nenhuma ferramenta/site
real, pasta pessoal, push/release ou snapshot. Próxima aprovação segue v1.3.1.

## Edição de texto e cancelamento legível (não publicado)

Campos nativos agora permitem mover cursor, inserir no meio, Home/End,
Delete/Backspace, Unicode e viewport horizontal. CSI completos não viram texto.
ask usa leitura nativa síncrona para restaurar raw antes do retorno por sinais.
Ctrl+C mostra Cancelado e resultado de cleanup, conservando código 130/causa.
Gates completos finais PASS; oito PTYs sintéticos PASS, incluindo nome editado,
Ctrl+C/SIGTERM durante digitação, restauração e demais fluxos. Shuffle expôs
suposição de ordem num teste do prefetch; readiness de todas as entradas passou
a ser aguardada. Seis cross-builds e smokes PASS; binário dist atualizado.
Nenhuma pasta/site/conta real alterada, nenhum push/release/snapshot.

## Release v1.3.1 autorizada — em andamento

Usuário autorizou push e publicação. Preview: suíte 1.3.0 -> 1.3.1,
media-get 0.2.0 -> 0.2.1 experimental; demais produtos permanecem 1.1.0.
main remoto corresponde a 14dfc950; tag/release v1.3.1 inexistentes na verificação.
Enviar implementação, aguardar CI nativo, preparar manifests/changelogs,
validar/reconstruir dist, enviar commit preparado e aguardar novo CI antes da tag.

## Release v1.3.1 — implementação verde e versão preparada

Commit 80a20415a560647d20c5ed778f3562ebcf37fbf7 enviado para main.
CI nativo 36944693037: success nos seis runners e todos os gates auxiliares.
https://github.com/matheusvcouto/cli-tools/actions/runs/36944693037
release prepare --suite-version v1.3.1 --write concluído: media-get 0.2.1
experimental, demais produtos 1.1.0; records arquivados em changes/archive/1.3.1.
Próximo gate obrigatório é CI do commit preparado antes de tag/publicação.
