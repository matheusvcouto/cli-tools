# Media Get — importação JSON e downloads em lote

Data: 2026-10-04. Estado: proposta para implementação; nenhum comando abaixo
existe ainda. Este pedido autoriza investigação/plano, não implementação,
commit, publicação ou escolha de versão. UI atual preservada.

Atualização de 2026-10-04: usuário aprovou a implementação local e informou
suíte 1.3.5 após sua aprovação. Os comandos abaixo agora estão implementados;
o restante registra a proposta original. Validação e ajustes entregues em
[BATCH_VALIDATION.md](BATCH_VALIDATION.md). Publicação e CI remoto pendentes.

## 1. Arquivo analisado e lacunas reais

Leitura somente de `<local-path>.json`, sem acessar
suas URLs, baixar mídias ou copiar conteúdo real para fixtures.

O documento tem `pageUrl`, `exportedAt` e 15 entradas em `videos`. Cada entrada
contém `title`, `url` HLS (`playlist.m3u8`), `referer`, `origin`,
`refererProvenance: inferred`, `confidence: hint`, `sources` e `command` vazio.
Não há versão, destino, qualidade, formato nem política de execução.

É um bom **manifesto de descoberta**. Títulos/URLs/headers permitem preparar
pedidos de vídeo na melhor qualidade, usando defaults explícitos. Não provam
que as playlists existem ou são acessíveis: `poster`/`preconnect` e confiança
`hint` são pistas, não captura comprovada de uma resposta de mídia.
Inspeção do backend continua necessária; JSON não substitui metadata.

`origin` ainda não está em `mediaget.Source`, que só suporta URL, Referer e
fragmentos. Ignorá-lo silenciosamente seria incorreto. `command` é dado legado:
nunca executar, interpretar como shell nem transformar em argumentos livres.
`pageUrl` é proveniência, não destino e não autoriza inventar headers ausentes.

## 2. Recomendação de entrada e compatibilidade

Usar um arquivo JSON local como entrada principal. Mais preciso que CSV ou URLs
separadas por vírgulas, pois preserva nome, headers e opções por item. Lista
de URLs por linha pode ser outro adapter futuro do mesmo modelo de lote.

Superfície proposta, declarada uma única vez no CLI Core com stable IDs:

```sh
media-get batch lote.json --output-dir "$HOME/Downloads"
media-get batch lote.json --jobs 3 --concurrent-fragments 8
media-get batch lote.json --yes
media-get batch validate lote.json
media-get batch schema
media-get batch example
```

`validate` é offline: estrutura, versões e semântica sem acessar mídia,
yt-dlp, HOME ou destino. Não promete download. `schema` imprime o JSON Schema
do manifesto; é distinto de `__cli schema`, que descreve a CLI. `example` imprime
um documento mínimo sintético para copiar. Não escreve arquivo implicitamente.
Help/version/completion estática/schema continuam sem probes de domínio.
Não capturar stdin nesta primeira entrega; arquivo evita disputar stdin com menus.

Aceitar explicitamente dois formatos:

1. Legado reconhecido pela estrutura `videos`, sem `schemaVersion`: importar
   os campos conhecidos do exportador; mostrar aviso de descoberta inferida.
   `title` fornece nome; `referer`/`origin` são preservados. `command` é ignorado
   com aviso se não vazio. Campos operacionais desconhecidos não são aceitos.
   Campos de proveniência conhecidos ficam separados das opções de execução.
2. Nativo `schemaVersion: 1`, `items`: contrato estrito com JSON Schema
   Draft 2020-12. Versão futura desconhecida é erro, não tentativa de adivinhar.

Não chamar o legado de v1 nem o nativo de v2: o arquivo recebido não declara
versão. O inteiro versiona o manifesto, independente da suíte/produto e do Go.
Não alterar caminho do módulo ou inventar URLs públicas de schema ainda inexistentes.

## 3. Contrato nativo proposto

Exemplo ilustrativo com dados sintéticos (ainda não é schema implementado):

```json
{
  "schemaVersion": 1,
  "defaults": {
    "kind": "video",
    "quality": "best",
    "videoFormat": "auto"
  },
  "execution": {
    "jobs": 2,
    "onError": "continue"
  },
  "items": [
    {
      "id": "ad-01",
      "url": "https://cdn.example.invalid/video-a/playlist.m3u8",
      "name": "Apresente seu produto",
      "referer": "https://example.invalid/pagina",
      "origin": "https://example.invalid"
    },
    {
      "id": "ad-02",
      "url": "https://cdn.example.invalid/video-b/playlist.m3u8",
      "title": "Outro anúncio",
      "options": { "videoFormat": "mp4" }
    }
  ]
}
```

Semântica a congelar antes de codificar:

- `schemaVersion` obrigatório, inteiro 1. `items` não vazio, até 1000 itens;
  documento UTF-8 até 8 MiB, profundidade até 32. Limites são operacionais
  propostos, não restrições do JSON ou medidas do computador.
- `id` opcional, único quando presente; ausente recebe identificador ordinal
  desta execução. `url` obrigatório HTTP(S), sem userinfo/controles.
- `name` é nome base solicitado; `title` é título informado pelo exportador.
  Precedência: `name` > `title` > título consultado. Sem nenhum deles, pedir
  nome em TTY; em `--yes`, erro por item antes do download, sem nome inventado.
  Nome informado evita repetir pergunta; saneamento aparece na revisão.
- `defaults`: `kind` video/audio/subtitle; `quality` best ou limites já aceitos
  na CLI; `videoFormat` auto/mp4; `subtitleFormat` srt/txt; `subtitleLang`;
  `autoSubs`; `concurrentFragments`; `outputDir`. `options` por item permite
  os mesmos campos. Legenda exige idioma; opções incompatíveis com kind falham.
- Defaults ausentes: vídeo, melhor qualidade, contêiner automático, 4 fragmentos
  ou env existente, destino pela resolução atual da CLI. Não converter para MP4
  automaticamente: recodificação custa CPU e pode alterar qualidade. MP4 explícito
  usa o fluxo compatível existente; áudio permanece M4A, legenda SRT/TXT.
- Opções são substituídas por campo, não por objeto inteiro. Valores opcionais
  usam representação de presença, distinguindo ausente de false/zero/vazio.
  Ausente herda; null é recusado; Referer vazio remove header herdado.
- Precedência efetiva: edição na revisão > flag explícita de lote > item >
  defaults do JSON > env suportada > default do programa. Flags globais são
  overrides deliberados, documentados no resumo. `--name` global é recusado
  para múltiplos itens. Extensão .mp4/.txt/.srt preserva a política atual de
  OutputSelection, prevalece no pedido efetivo e aparece na revisão.
- Referer/Origin opcionais em defaults e item; aceitar Origin como origem
  HTTP(S) sem usuário, path além de `/`, query ou fragmento. Validar CR/LF.
  Não oferecer mapa genérico de headers/cookies/tokens/flags do backend.
- `execution.jobs`: 1..8, padrão 2. `onError`: continue (padrão) ou stop.
  Novas env não são necessárias inicialmente; flag pode substituir execution.
- `metadata` opcional para proveniência (`pageUrl`, `exportedAt`, confiança,
  fontes); nunca controls de execução. Propriedades desconhecidas fora desse
  espaço devem falhar; extensão futura exige atualização explícita do contrato.
- Names são bases, não paths; destino relativo resolve contra cwd, como hoje.
  Evitar semântica implícita relativa ao JSON. `~` no arquivo não é expandido;
  exemplo gerado usa ausência de destino ou caminho absoluto informado.

Evitar o termo "definitivo" como promessa de não mudar: ter versão, schema,
exemplos e regras de evolução torna o formato durável. Mudança incompatível
futura tem outra schemaVersion; leitor antigo recusa com diagnóstico claro.

## 4. Resumo e fluxo sem refazer a UI

Pipeline: ler/validar tudo offline → resolver defaults → verificar dependências
e destinos → consultar metadata com concorrência limitada → resolver nomes e
seleções → mostrar todos os itens → Baixar / Editar / Cancelar → executar.

Nenhum arquivo de mídia é criado antes da confirmação; metadata pode acessar
rede e seu cache continua somente em memória. Títulos do JSON não dispensam
inspeção da qualidade/faixas/acessibilidade. No legado, ausência de configuração
usa vídeo/best/auto e destino atual, apresentados no resumo.

Resumo em stderr reaproveitando os componentes atuais: posição/id, nome final
previsto, formato/extensão, qualidade/idioma, destino, estimativa ou desconhecida,
Referer/Origin definidos, avisos de inferência, quantidade total e jobs/fragmentos.
Auto diz "extensão a confirmar". Totais só somam estimativas conhecidas e
informam quantos itens faltam; não tratar desconhecido como zero.
Conteúdo longo não deve omitir itens: texto por item ou páginas simples.
Ocultar URLs/headers como no fluxo atual; sanear controles dos títulos.

Se um item falhar na preparação, bloquear a confirmação até o usuário
corrigir, excluir explicitamente ou cancelar. Em --yes, preparação inválida
aborta o lote antes de qualquer transferência. Depois de iniciar, onError
governa falhas de execução. Sem downgrade silencioso da qualidade escolhida.
Edição de nome/destino/fragmentos reaproveita metadata; alteração de fonte,
Referer/Origin/seleção invalida apenas o cache afetado.

Nomes previstos não são reservas: colisões externas ainda podem ocorrer.
Publicação mantém no-clobber; mostrar paths realmente entregues no resultado.
Pré-calcular bases distintas de itens com títulos iguais na ordem da lista
reduz variação quando downloads acabam fora de ordem, sem substituir arquivos.

## 5. Simultaneidade, falhas e processamento

`jobs` limita arquivos em execução; `concurrentFragments` limita fragmentos de
cada download HLS/DASH. 3 jobs × 8 fragmentos aproxima 24 operações de fragmentos,
mas streams separados e outros processos podem elevar o total. Não é teto
global de sockets nem promessa de ganho de velocidade.

Começar com 2 jobs; manter faixa 1..8 e fragmentos 1..256 existente. Resumo
avisa combinações grandes. Inspeção tem pool de até 3 workers para todo o lote,
sem multiplicar o prefetch de 3 workers por cada mídia. Conversão FFmpeg é
limitada a 1 por lote inicialmente, cancelável durante a espera; baixar outro
item pode continuar. Medir antes de ampliar esses defaults.

Scheduler entrega índices da lista, eventos imutáveis por item e estados:
preparando, pronto, baixando, processando, concluído, falhou, cancelado, não iniciado.
Um único renderer recebe eventos; workers nunca escrevem simultaneamente
stderr nem fazem perguntas. UI futura poderá consumir os mesmos estados sem
reescrever o domínio. Primeira UI usa linha agregada + conclusão de cada item;
redirecionamento recebe logs limitados. Não fingir percentual de conversão.

Reutilizar Service.Download e sua publicação confinada por pedido, sem passar
todas as URLs a um único yt-dlp --batch-file: nomes/headers/opções/falhas são
por item e a área privada atual exige um único resultado válido. Adaptar a
fronteira de processamento somente onde necessário para o semáforo de FFmpeg;
não duplicar download/conversão/publicação para criar batch.

onError=continue conclui demais itens. stop deixa de iniciar novos, permite
concluir os que já executam e retorna falha. Ctrl+C cancela os contextos e
grupos de processos de todos os ativos, recolhe workers, restaura terminal e
preserva entregas concluídas. Incompletos são tratados pelo coordenador,
sequencialmente, pela política atual; --yes descarta só áreas desta execução.
Retenção/cancelamento nunca varre downloads anteriores.

Stdout continua um path entregue por linha; ordem de conclusão é documentada.
stderr traz resultados na ordem da lista. Exit codes alinhados ao core:
0 se todos concluírem, falha se houver qualquer erro, 130 no cancelamento.
Relatório JSON estruturado e retomada persistente podem vir depois; não manter
manifesto real/URLs assinadas automaticamente em logs ou relatórios.

## 6. JSON v2, schema e dependências

JSON do arquivo, schemaVersion e encoding/json/v2 são assuntos distintos.
Na instalação Go 1.27.1 lida nesta máquina, `encoding/json/v2/doc.go` possui
build constraint `goexperiment.jsonv2`. Não ativar GOEXPERIMENT nem migrar o
projeto inteiro por esta funcionalidade. `encoding/json` é suficiente para
ler tipos/presença, com validação estrita adicional de duplicatas, casing exato,
UTF-8, trailing data, tamanho/profundidade e propriedades desconhecidas.
DisallowUnknownFields sozinho não cobre todas essas garantias.

Para validar o schema completo, candidato recomendado a avaliar na implementação:
`github.com/santhosh-tekuri/jsonschema/v6` (v6.0.3 pesquisada), Draft 2020-12,
Apache-2.0, Go mínimo 1.21; go.mod inclui x/text e regexp2 marcado para testes.
Isso não altera o path do nosso módulo: o /v6 pertence à dependência.
Auditar a versão fixada, grafo transitivo real, manutenção, advisories e notices
antes de adicionar; nenhuma dependência foi instalada nesta investigação.

Lacuna motivadora: aplicar o mesmo schema publicável que produtores externos
copiam, sem manter um validador JSON Schema caseiro. Compilar schema embutido,
sem loader HTTP/$ref remoto. Validação semântica usa regras existentes de
Source/Selection; testes de paridade evitam divergir schema e structs.
Um gerador de schema por reflection não é necessário inicialmente: uniões e
combinações de kind precisam contrato explícito. Não adicionar framework de fila,
Cobra/Viper, YAML ou outra biblioteca de TUI para esta etapa.

## 7. Geração pelo navegador / Google

O produtor externo gera o manifesto, o media-get o valida e executa. Entregar
schema, exemplo mínimo e instrução copiável de exportação, para usar no Chrome,
num exportador ou numa ferramenta de geração. O nome "Google" não identifica
se é Chrome/DevTools, Gemini ou outro produto; não assumir integração/conta.
Confirmar qual produtor antes de escrever código específico para ele.

Instrução sugerida ao produtor: "Gere schemaVersion 1, defaults video/best/auto
e items com url e title/name. Copie URLs e headers observados; não invente
playlists, origem nem credenciais. Se a URL for inferida, registre isso em
metadata. Não gere command, cookies ou shell. Não defina destino local sem
instrução do usuário. Valide contra o schema fornecido."

Schema não descobre mídias nem garante que um modelo gerador produza URLs
reais. Um exportador que só vê poster/preconnect precisa declarar a inferência;
o backend decide acessibilidade. Exportador/browser fica fora desta entrega.

## 8. Ordem de implementação e critérios de aceite

- [ ] Congelar nomes/tipos/defaults/precedência e schema nativo v1; exemplos
  inteiramente sintéticos e adapter legado com mapeamento/avisos testados.
- [ ] Parser/validador dedicado em internal/mediaget, fronteira de importação
  separada do domínio; limite de bytes, duplicatas/casing/versão e diagnóstico
  com índice/campo, sem reproduzir URLs sensíveis.
- [ ] Origin tipado no Source, validado e repassado como argv
  `--add-headers Origin:<origem>` em todas as consultas/downloads do adapter.
  Cache inclui Origin/Referer; regressões de header injection e invalidação.
- [ ] Preparação do lote, resolução de nomes/destinos, resumo/edição e validação
  completa antes da transferência. Campo formato/extensão cobre pendência atual.
- [ ] Scheduler cancelável com jobs, pool global de metadata, limite FFmpeg,
  eventos serializados e resultados estáveis. Backend e Service.Download
  reutilizados; concorrência não relaxa containment/identidade/no-clobber.
- [ ] App declarativa batch/validate/schema/example, --yes e flags compatíveis,
  help/completion/contract gerados, sem alterar comportamento single-URL.
- [ ] Testes offline: importação de 15 itens sintéticos; nomes ausentes;
  defaults/overrides/false/vazio; tipos/versões/duplicatas/UTF-8 inválidos;
  arquivos grandes; comandos ignorados; header injection; jobs efetivos;
  conversão limitada; cancelamento de todos os descendentes; falhas parciais;
  preparação sem mutação; colisões concorrentes/symlinks; falha de renderização;
  destinos inválidos; erros sem vazamento; formato/extensão; stdout/exit codes.
- [ ] README/ADR/schema/change record/gates check-safe.sh all e build nativo
  dist/media-get/media-get, help/version isolados; CI nativo conforme regras.
  Testes usam somente fakes/fixtures sintéticas, nunca o arquivo recebido.
- [ ] Usuário testa a implementação antes de decidir publicação. Calcular
  impactos de produto/suíte e perguntar explicitamente se divergirem da versão
  esperada. Não selecionar número de release neste plano.

Primeira entrega: JSON + legado + vídeo/áudio/legenda com uma saída por item,
lote concorrente e resumo atual. Seleção múltipla por barra de espaço é tarefa
separada. Evolução para múltiplas saídas usa expansão em pedidos identificados
sem sobrecarregar este contrato com funcionalidade ainda não implementada.

## 9. Fontes e limites da investigação

Fontes oficiais consultadas em 2026-10-04:

- JSON Schema [Draft 2020-12](https://json-schema.org/draft/2020-12)
  define a descrição/validação do documento; não valida mídia remota.
- Go [encoding/json/v2](https://pkg.go.dev/encoding/json/v2) e
  [anúncio experimental](https://go.dev/blog/jsonv2-exp); baseline também
  conferida no source da instalação local, sem executar experimentos.
- Validador [v6.0.3](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6@v6.0.3),
  [go.mod](https://github.com/santhosh-tekuri/jsonschema/blob/v6.0.3/go.mod) e
  [licença](https://github.com/santhosh-tekuri/jsonschema/blob/v6.0.3/LICENSE).
- yt-dlp [opções oficiais](https://github.com/yt-dlp/yt-dlp#usage-and-options):
  batch-file, concurrent-fragments e headers. Fragmentos não são concorrência
  entre arquivos; wrapper continua responsável por planejamento/publicação.

Revisão de arquivo/código/docs e proposta arquitetural, não evidência de
download/runtime/desempenho. Nenhuma URL de mídia acessada, dependência
instalada, implementação aplicada ou release publicada nesta etapa.
