# Lote JSON — implementação local para teste

2026-10-04. Usuário aprovou a implementação e escolheu suíte futura 1.3.5,
publicável somente após seu teste/aprovação. Nenhum commit/push/tag/release
executado nesta etapa. Binário: `dist/media-get/media-get`.

## Entregue

- Manifesto nativo schemaVersion 1 e importação do formato videos, com nome,
  URL, Referer/Origin. Comandos legado jamais executados.
- Schema Draft 2020-12 e exemplo embutidos; validate offline sem probes/HOME.
- Parser UTF-8 limitado a 8 MiB/32 níveis/1000 itens, rejeitando duplicatas,
  casing inválido, null, lone surrogates, tipos e versões incompatíveis.
- Defaults/overrides por presença, flags > item > JSON > env > programa;
  nome > título informado > metadata; pergunta somente se tudo estiver ausente.
- Preparação com até três workers, validação de todos os pedidos antes de
  transferir e bloqueio de --yes inválido. Resumo/edição/exclusão por item.
- Scheduler 1..8 jobs (default 2), resultados por item, stop/continue,
  callbacks serializados, cancelamento/join e publicação existente sem clobber.
- Formato/extensão no resumo de downloads individuais e lote; automático
  informa extensão desconhecida. Nomes previstos não reservam destinos.
- Slot cancelável para conversão **adicional** MP4 compatível. FFmpeg interno
  do yt-dlp ainda limitado por jobs: ajuste documentado em M019/README.
- README, ADR, notices das dependências, contrato da CLI e change record.

## Evidência local

`scripts/check-safe.sh all`: PASS (fmt/test/vet/shuffle/race/API/contracts/changes).
Após os últimos ajustes de escrita curta/entrada de arquivo, race de todos os
pacotes internal/mediaget: PASS, incluindo os novos testes. Logs em
`dist/batch-validation/{all,final-all,final-race}.log`.

Build CGO_ENABLED=0 nativo macOS ARM64: PASS; help/version, batch help,
schema/exemplo e validate do exemplo sintético em ambiente isolado: PASS.
Binário usa produto do manifest atual 0.3.1 e suite_version v1.3.4+dev:
é build de desenvolvimento, não release 1.3.5 preparada.

Dependências fixadas: jsonschema/v6 6.0.3 (Apache-2.0), x/text 0.42.0
(BSD). go mod tidy, check-safe prepare e go mod verify: PASS. regexp2
1.11.0 é dependência de teste do validador; não há import runtime no binário.
Source/licenças/loader revisados; compiler.UseLoader(nil) desativa o fallback
de arquivo da biblioteca. Schema/meta-schema embutidos. Não foi executado
scanner de advisories/govulncheck; não afirmar auditoria de segurança integral.

Fixtures já sintéticas. Nenhuma URL, mídia ou conta real utilizada nos testes.
Arquivo lote.json do usuário preservado e não usado como fixture.
CI remoto, testes de site/throughput real e novos runners nativos: não executados.
Windows continua sem download; não promover suporte por compilação local.

## Pendência de versionamento antes de publicar

Preview somente leitura de prepare --suite-version 1.3.5 recusou a versão:
a regra atual agrega o maior impacto de qualquer produto e calcula 1.4.0.
Record correto do recurso é minor: media-get 0.3.1 → 0.4.0; demais produtos
permanecem 1.1.0, API Go pública preservada. Não materializar 1.4.0 nem reduzir
o impacto do record para contornar o tooling.

Pergunta explícita enviada ao usuário para separar o cálculo da suíte:
mudanças privadas das CLIs avançariam a suíte em patch; impactos do módulo/API
continuariam exigindo minor/major. Aguardar decisão antes de alterar o tooling
ou prepare --write. Depois, obter aprovação do teste para publicar 1.3.5.

Pendências funcionais em tasks.md: seleção múltipla por espaço e entrada de
URLs por linhas; não estão autorizadas por este pedido de implementação JSON.

## Correção de destino após teste do usuário

O destino ausente deixou de ser erro: validação read-only aceita subpastas
ausentes sob um ancestral real. A transferência confirmada cria o caminho com
safefs.EnsureDir; preparação/cancelamento antes de baixar não cria pastas.
Arquivo/symlink existente continua sendo rejeitado, sem substituição.
Fluxo individual e lote compartilham a mesma criação segura. Edição do destino
mantém metadata; jobs simultâneos podem criar o destino comum sem colisão.

Regressões sintéticas de criação aninhada/concorrente, edição, cancelamento,
pedido inválido e ancestrais file/symlink: PASS. check-safe.sh all após a correção:
PASS; log dist/batch-validation/destination-all.log. Contrato regenerado, build
nativo reconstruído e batch help/version isolados PASS. Nenhuma pasta real em
Downloads foi criada ou usada nos testes. Publicação continua pendente.

## Edição de formato e painel de progresso

Edição direta de formato preserva qualidade/idioma; opção do lote aplica MP4
compatível aos vídeos incluídos. Indicadores mostram validação/consultas, e o
painel reaproveita slots de itens ativos. Barra geral conta itens finalizados
(incluindo falhas), enquanto transferência/conversão/publicação são individuais.
A espera pelo conversor MP4 também tem indicador animado.

Regressões de edição individual/global, slots, largura, animação, logs sem ANSI
e cancelamento/join após falha do terminal PASS. check-safe.sh all PASS
(incluindo race, API e contratos). Build nativo e help/version isolados PASS.
Evidências: dist/batch-validation/progress-all.log, progress-help.log e
progress-version.json. Não houve download real nem CI remoto.

## Aprovação de publicação — 2026-10-08

Publicação suite 1.3.5 aprovada explicitamente. Política de cálculo independente
implementada e documentada: CLI privada pede patch da suíte, module continua
definindo minor/major. Media-get preparado como 0.4.0, demais produtos 1.1.0.
A pendência de numeração acima está resolvida. Dados do manifesto pessoal e
caminhos locais foram substituídos por exemplos genéricos; builds com trimpath.
