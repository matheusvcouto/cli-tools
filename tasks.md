# Tarefas

## Prioridade alta — configuração central da suíte

Visão em [ideias.md](ideias.md#configuração-central-da-suíte) · decisão no
[ADR D024](ADR.md#d024--diretório-central-por-ferramenta--accepted).

- [x] **toolhome:** pacote `internal/toolhome` resolve `~/.cli-tools/<tool>/`
  (ou `$CLI_TOOLS_HOME/<tool>/`) pelo nome do `tool.json`, com layout fixo
  (`config.toml`, `state/`, `cache/`, `backups/`), criação sem seguir symlinks e
  `EnsureConfig` que cria o padrão uma vez e nunca sobrescreve.

- [ ] **skills:** nova CLI usa `toolhome` desde o início — config em
  `~/.cli-tools/skills/config.toml`, índice de destinos em `state/`, backups em
  `backups/` com retenção de 30 dias. Plano em `plans/skills/` (em elaboração).

- [ ] **Migração das CLIs existentes:** levantar o que `ai-profile`
  (`~/.ai-profiles`, `AI_PROFILE_ROOT`) e `media-get` guardam hoje e propor,
  para cada uma, como migrar para `toolhome` sem perder dados nem quebrar
  quem já usa (leitura do local antigo, migração explícita ou manter). Exige
  decisão antes de implementar.

## media-get

Plano de investigação para entrada JSON e execução concorrente do lote:
[plans/media-get/BATCH_JSON.md](plans/media-get/BATCH_JSON.md).
Implementação publicada na suíte 1.3.5 (media-get 0.4.0).

- [x] **media-get:** formato/extensão explícitos no resumo, nome previsto com extensão quando conhecida e aviso de extensão a confirmar no automático. Colisões recebem sufixo; o nome previsto não é uma reserva.

- [x] **media-get:** lote por manifesto JSON nativo ou export `videos`, com nomes, opções comuns/por item, revisão/edição/exclusão, concorrência limitada, falhas parciais e cancelamento. Schema/exemplo/validação offline disponíveis; publicado na suíte 1.3.5.

- [x] **media-get:** edição direta do formato de saída, aplicação de MP4 ao lote, indicadores nas consultas e painel de progresso com barra geral e barras por download ativo. Publicado na suíte 1.3.5 após validação local e CI nativo.

- [ ] **media-get — próxima versão:** permitir selecionar múltiplas saídas no menu de download com a barra de espaço, navegar com as setas e confirmar o conjunto com Enter. Exemplos: vídeo + áudio, vídeo + legenda ou vídeo + áudio + legenda. Após a seleção, configurar os formatos e demais opções de cada saída, como qualidade do vídeo e idioma/formato da legenda, e mostrar todas as saídas no resumo final. Executar o conjunto a partir da mesma URL, sem exigir que o usuário repita o fluxo para cada arquivo. Manter a seleção de uma única saída disponível e definir/testar cancelamento e falhas parciais do conjunto antes da publicação.

- [ ] **media-get:** entrada adicional de lote como lista de URLs por linhas, reutilizando o planejador/fila já implementados para JSON. Avaliar um separador explícito como vírgula sem quebrar URLs que contenham esse caractere. Oferecer opções comuns interativas (tipo/formato/qualidade/destino), títulos originais ou nomes individuais e exclusão de itens na revisão. Manter os testes de cancelamento, falhas e colisões do lote.
