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
