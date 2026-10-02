# Media Get — plano e acompanhamento

## Objetivo e origem

Trazer as capacidades úteis de `/Users/matheus/pessoal/videos` para esta suíte,
com domínio Go, CLI Core único e backend yt-dlp substituível. A referência Deno
foi lida somente como fonte de comportamento. Banco, mídias, arquivos de conta e
configurações reais não são copiados, executados, alterados ou usados em testes.

O exemplo `yt-dlp -F --referer ... main.m3u8` foi interpretado como **Referer**,
o header da página de origem. Não foi interpretado como reparar um arquivo
corrompido. O link remoto não foi reproduzido nem usado como teste. Um comando
`repair` com outra semântica deve ser especificado separadamente.

## Contrato proposto e implementado

- Nome `media-get`; manifest inicial experimental `0.1.0`, sem mudar versões
  de ferramentas existentes ou o módulo.
- Interativo: URL → vídeo com áudio / áudio / SRT →
  qualidade ou faixa (com estimativas nos menus) → transferência estimada → continuar/ajustar/cancelar/configurar →
  nome → confirmação. Menus com setas/busca/Enter em TTY Unix e numerados no fallback; voltar em qualidade/legenda.
- Automatizável por `--kind`, `--quality`, `--subtitle-lang`, `--auto-subs`,
  `--name`, `--referer`, `--output-dir` e `--yes`. Sem TTY, exigir URL/kind/yes
  antes de acessar a rede ou escrever.
- Destino: flag > `CLI_TOOLS_MEDIA_GET_DOWNLOAD_DIR` > `~/Downloads`.
  Variável vazia é erro; pasta deve existir e não pode ser raiz symlink.
- Metadados e estimativa vêm de seleção real do backend. Soma de todos os
  streams; duração/bitrate somente como aproximação. Ausência de qualquer
  parcela significa total desconhecido. Falha de estimativa não bloqueia;
  cancelamento não pode ser confundido com estimativa indisponível.
- Áudio M4A, legenda SRT, vídeo com extensão realmente entregue (merge MP4/MKV).
  Nome saneado; colisões recebem sufixo sem substituir qualquer arquivo.
- Erros preservam causas internas; dependência ausente indica ferramenta e
  instalação. Processo informa etapa e exit code sem expor stderr/URLs.

## Arquitetura e ordem de execução

1. **Levantamento:** ler regras, docs canônicas e script Deno; separar capacidade
   necessária de histórico/conversões opcionais. Sem instalar bibliotecas.
2. **Domínio:** Source/Selection/Info/Request/Progress/Result e Backend;
   estimativa, seleção válida, nome, raiz de destino, publicação create-exclusive
   e recuperação. Domínio não conhece SO, shell, JSON ou selectors yt-dlp.
3. **Adapter:** JSON yt-dlp, selectors por tipo/altura, Referer em todas as
   consultas/downloads, dependências por operação, ambiente mínimo e processos.
   Diferenças de cancelamento ficam no backend de plataforma.
4. **CLI:** uma App declarativa com stable IDs e codecs/constraints; interação
   pelo port do core. Help/version/schema/contract/completion livres de probes
   de domínio, HOME e executáveis. Stdout caminho final; stderr interação.
5. **Segurança e falhas:** pasta privada no filesystem do destino, identidade
   física revalidada, arquivo único regular/não vazio, publicação por hard link.
   Em erro descartar por padrão, com retenção interativa explícita; nunca sobrescrever ou apagar destino antigo.
6. **Testes:** fixtures já sintéticas, nenhum download real. Rodar testes de
   estimativa, fluxo, dependências, argv/env, falha, cancelamento de descendentes,
   arquivos/symlinks adversariais, colisões e concorrência; congelar contrato.
7. **Entrega:** docs/ADR/change record e gates herméticos.
   Entrega na worktree persistente; sem snapshots de transferência.

Layout:

```text
cmd/media-get/                 entrypoint, manifest, docs, contrato
internal/mediaget/              domínio e publicação
internal/mediaget/cli/          Spec e fluxo via Interaction
internal/mediaget/ytdlp/        adapter e backends de processo
plans/media-get/                plano e evidências da rodada
```

## Dependências e plataformas

| Operação | Dependências externas |
| --- | --- |
| Help/version/schema/completion | nenhuma |
| Consultar metadados/estimar | yt-dlp |
| Vídeo com áudio/merge | yt-dlp + ffmpeg |
| Áudio M4A | yt-dlp + ffmpeg + ffprobe |
| Legenda convertida SRT | yt-dlp + ffmpeg |

Recomendação contextual: `brew install yt-dlp ffmpeg` no macOS;
`sudo apt install yt-dlp ffmpeg` em Debian/Ubuntu, com aviso na documentação de
que versões antigas do pacote podem não oferecer as flags necessárias. Não
instalar automaticamente nem habilitar download de componentes remotos. EJS e
runtime JS para certos extratores são requisitos adicionais do yt-dlp, descritos
nas fontes oficiais; não são garantidos apenas por achar o executável.

macOS é o alvo inicial. Linux compartilha processo Unix e dica apt, mas precisa
de runner nativo para cada revisão; nesta tarefa o launcher passou no CI nativo.
A pasta XDG personalizada requer flag/env por enquanto.
Windows possui somente superfície estática: download fail-closed até implementar
contenção nativa de processos e confirmar suas capabilities em testes reais.

## Invariantes de segurança

- Config/plugins/cache do yt-dlp e componentes remotos desativados. Netrc é
  opt-in no parser oficial e não é solicitado. Nenhum cookie/credential reader.
- Só PATH/locale no ambiente filho. Sem shell intermediário, argv literal.
- URL/Referer sensíveis no contrato; nunca persistidos em histórico/logs próprios.
- Progresso aceita apenas template numérico; saída bruta do filho é descartada.
- Timeout de metadata 45 s, socket 20 s, JSON até 8 MiB. Download sem timeout
  total, mas cancelável; Unix mata o grupo dedicado inclusive FFmpeg.
- Backend do sistema é confiável; `os.Root` confina publicação e limpeza, não
  constitui sandbox para código executado pelo binário externo.
- Validar origem/destino antes de mutar e nunca tratar alias textual como
  identidade. Hard link deve ser suportado, caso contrário retornar erro.

## Evolução posterior

Histórico SQLite, TXT, transcodificação H.264 forçada/QuickTime, retomada, limpeza
assistida, playlists, cookies e seleção de múltiplos áudios ficam
para decisões próprias. A primeira versão não precisa de library TUI, registry
por site ou abstração global. Backend futuro recebe os mesmos modelos e deixa
publicação e interação intactas; acrescentar um resolver de site somente quando
existir o segundo adapter e seu comportamento puder ser testado.

## Checklist da rodada

- [x] Leitura da referência e documentação do projeto.
- [x] Domínio, backend yt-dlp, plataforma e CLI declarativa.
- [x] Destino por flag/env/default e Referer opcional.
- [x] Vídeo/áudio/SRT, estimativa tolerante e progresso.
- [x] Publicação sem clobber, concorrência e preservação de incompletos.
- [x] Testes focados com backend/processos sintéticos passaram na primeira execução.
- [x] Revisão oficial corrigiu flag inexistente `--no-netrc` antes da entrega.
- [x] README/ADR/manifest/change record adicionados.
- [x] Gates completos após a última revisão e contrato final: PASS.
- Evidência histórica: artifacts da rodada 009 foram registrados à época;
  o fluxo vigente não gera snapshots, contextos separados ou Base64.
- [ ] Validação opt-in com mídia sintética e yt-dlp/FFmpeg reais no macOS.
- [x] Launcher Linux nativo (x64/ARM64): CI `36933364310` PASS.
- [ ] Downloads Windows: capability ainda não implementada.

Resultados finais e pendências ficam em `VALIDATION.md` e `CONTEXT.md` da rodada.

## Complemento — prévias e progresso

- [x] Estimativas de vídeo/áudio antes da escolha e por limite de qualidade.
- [x] Até três consultas paralelas com goroutines, orçamento de 20 s por lote,
  cache local e tolerância a tamanho desconhecido.
- [x] Carregamento animado/contador e progresso de stream com speed/ETA;
  etapas explícitas de processamento e publicação.
- [x] Gates da atualização: testes/vet/shuffle/race/API/contratos/changes PASS;
  resultado em VALIDATION.md.

## Complemento — experiência interativa (incluído em v1.3.1)

- [x] Menus nativos com setas/busca/Esc e restauração do terminal.
- [x] Referer opcional na revisão; recuperar consulta inicial bloqueada.
- [x] Atualizar metadados/cache/seleção após editar Referer.
- [x] Configuração fechada de fragmentos paralelos (1 a 8, default 1).
- [x] Barra na mesma linha; total aproximado/desconhecido explícito e animação
  durante processamento/publicação; logs simples fora de TTY.
- [x] Dependências oficiais pequenas, fixadas em go.mod/go.sum; cache preparado e notices na release.
- [x] Gates completos, build nativo/smokes e cross-build; evidência em VALIDATION.md.
- [x] Usuário autorizou push/preparo/publicação de v1.3.1 em 2026-10-01.

## Revisão após execução do usuário

- [x] Corrigir parsing de totais/ETA HLS decimais e notação científica.
- [x] Levar previsão do wizard ao início da barra, com fallback conhecido.
- [x] Contabilizar streams por identidade opaca, sem duplicar eventos/bytes.
- [x] Preservar última linha de progresso em cancelamento/erro.
- [x] Compactar menus concluídos e evitar caps redundantes com altura conhecida.
- [x] Validar ffprobe efetivamente utilizado junto a FFmpeg.
- [x] Validação final, cross-build e rebuild/smokes; resultados em VALIDATION.md.
- [x] Usuário confirmou Ctrl+C: cancelamento/130 esperado, sem evidência de
  encerramento espontâneo.

## Descarte de incompletos (incluído em v1.3.1)

- [x] Descarte padrão após falha/cancelamento, sem tocar pastas antigas.
- [x] Escolha interativa após renderer/backend encerrarem; manter explícito.
- [x] Pasta visível, caminho/tamanho e identidade revalidada antes da limpeza.
- [x] Gates completos e teste nativo da decisão pós-Ctrl+C: PASS.

## Prefetch e escolha imediata (incluído em v1.3.1)

- [x] Vídeo/áudio começam com metadata; qualidades enfileiradas antes da seleção.
- [x] Menus sem espera de estimativas e labels/spinner atualizados em TTY.
- [x] Cache em memória até confirmação final; deduplicação e workers limitados.
- [x] Cancelamento/Referer recolhem probes; download sem consultas pendentes.
- [x] Gates completos, terminal lento sintético e rebuild/cross-build: PASS.

## Edição de campos de texto (incluído em v1.3.1)

- [x] Setas/cursor, inserção, Home/End, Backspace/Delete e viewport horizontal.
- [x] Restore síncrono antes de retornar Ctrl+C/SIGTERM em campo nativo.
- [x] Cancelamento legível com resultado de cleanup, mantendo causa/código 130.
- [x] Gates finais, edição real no PTY, cancelamento no nome e rebuild: PASS.

## Revisão de throughput após v1.3.2 (local, ainda não publicada)

- [x] Consultar yt-dlp e os dois repositórios citados sem instalar aplicativos.
- [x] Explicar a projeção HLS variável; preservar ≈ quando não houver total exato.
- [x] Dar prioridade a metadata/total real sobre amostras aproximadas posteriores.
- [x] Default 4 fragmentos, override 1..256 e flag validada antes de consultas.
- [x] Remover duplicação de previsão e resumo das configurações.
- [x] Gates completos, seis cross-builds e binário nativo/smoke isolado PASS.
- [ ] Medição opt-in de throughput real e CI remoto da nova branch.

Decisões/limites/fontes em ../../docs/media-get-throughput.md e M014.

- [x] Resumo final antes da transferência, flags sem pergunta de nome repetida (M015).
- [ ] Medição real de throughput pelo usuário antes de publicar a próxima release.

- [x] Preferência por env, flag > env > default, teto 256 e documentação (M016).

- [x] Menu final Baixar / Editar opções / Cancelar; edição local e resumo atualizado (M017).

## Correção de saídas — v1.3.4 autorizada

- [x] Escolha SRT/TXT após selecionar a faixa; flags e resumo.
- [x] TXT validado sem tempos/numeração/markup, mantendo números falados.
- [x] MP4 compatível opcional, extensões explícitas e revisão final.
- [x] Verificação de codecs/vídeo/áudio, conversão cancelável e sem clobber.
- [ ] Gates locais, builds, CI nativo e publicação v1.3.4.
