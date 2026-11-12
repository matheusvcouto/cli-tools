# ADR — notion-export

## N001 — Migração por capacidade da CLI Deno — Accepted (2026-11-12)

Reimplementar em Go o `notion-public-exporter` (Deno/TypeScript, 1.0.0), usado
como referência somente de leitura. Preservar capacidades e invariantes:
página pública sem token, Markdown + HTML (`--format md|html|both`), `page.json`,
`manifest.json` schemaVersion 1 com o mesmo layout camelCase, inventário e
perguntas por tipo antes de baixar, `--from-json`, downloads concorrentes com
validação de Content-Type e fallback remoto. Texto de help, mensagens e alguns
detalhes de saída podem mudar (paridade funcional, D013 da suíte).

Nome do produto: `notion-export` (verbo-substantivo, como `media-get`/`repo-zip`).
Começa em 0.1.0 `experimental`: depende de API não oficial. O manifesto aceita o
`generator.name` legado `notion-public-exporter` apenas para validar `--force`.

Domínio em `internal/notionexport`, adapter da API em
`internal/notionexport/notionapi`, composição em `internal/notionexport/cli`.
Renderers não fazem I/O. Sem estado persistente nem configuração: `toolhome`
(D024) não é usado.

## N002 — Adapter da API interna, sem `notion-client` — Accepted

O original usava `notion-client` (npm). Em Go, chamar diretamente os mesmos
endpoints, conferidos no código de `notion-client` 8.0.8/`notion-utils` e por
consulta real a uma página pública de demonstração em 2026-11-12:

- `POST /api/v3/loadPageChunk` (`pageId` UUID, `limit` 100, chunk 0);
- `POST /api/v3/syncRecordValuesMain` para blocos referenciados ausentes;
- `POST /api/v3/getSignedFileUrls` para arquivos hospedados pelo Notion.

Base `https://app.notion.com/api/v3`. Nenhum cookie/Authorization é enviado;
User-Agent genérico sem dados pessoais. Diferenças deliberadas:

- o loop de blocos ausentes do `notion-client` não termina quando o Notion
  nega um bloco; aqui cada ID é pedido uma vez, em lotes de 100, com no máximo
  25 rodadas;
- `queryCollection` (bases de dados) não é chamado: nenhum renderer as usa, e
  isso reduz a superfície da API não oficial. Collections aparecem como aviso;
- assinatura apenas das fontes de blocos de asset alcançáveis, em lotes de 50;
  falha na assinatura é aviso, não erro fatal;
- respostas limitadas a 64 MiB; recordMap salvo limitado a 256 MiB.

Retry igual ao `notion-client`: 3 tentativas para 408/429/500/502/503/504 e
erros de transporte, backoff exponencial com jitter (1 s a 60 s), cooldown de
60 s + jitter após 429, `Retry-After` limitado a 2 min; 4xx falha rápido como
página privada/inexistente. Todas as esperas respeitam cancelamento.

## N003 — HTML por escaping e allowlist de URL — Accepted

Herda os ADR-002/003/004 da CLI Deno: HTML montado a partir de campos
conhecidos, todo texto/atributo escapado no ponto de saída, nenhum HTML vindo do
Notion, nenhum script/iframe/object/form, CSS local embutido e CSP restritiva.
URLs aceitas: `http(s)` com host e sem userinfo, `mailto:`, fragmentos e
caminhos relativos sem `..`. Links internos `/<page-id>` do Notion viram
`https://www.notion.so/<id>`; os demais caminhos absolutos, protocol-relative,
`javascript:`, `data:` e caracteres de controle/bidi são descartados.

Markdown também escapa o texto do Notion (inclusive `<`/`>`), para que o
conteúdo não vire HTML bruto num visualizador. Títulos do Notion começam em
`##`, alinhados ao HTML (antes eram `#`). Mensagens de terminal removem
caracteres de controle vindos de títulos, nomes e erros remotos.

Nesting limitado a 64 níveis e ciclos de synced block interrompidos.

## N004 — Downloads — Accepted

Probe de tamanho por `GET` com `Range: bytes=0-0` (o host de arquivos responde
403 a `HEAD`), total por `Content-Range` ou `Content-Length`; o corpo nunca é
lido. Downloads: concorrência 1..64 (padrão 6), somente `http(s)`, até 10
redirects `http(s)`, Content-Type validado para imagem/vídeo/áudio/PDF,
comprimento conferido, 60 s sem dados abortam a transferência, até 2 retries
transitórios. Arquivo criado com `O_EXCL` na raiz confinada e removido se a
transferência falhar. Nome: `NNN-<nome portável>`, ASCII, até 100 bytes, sem
separadores/reservados; o índice garante unicidade. Vídeos/áudios de serviços de
streaming (YouTube, Vimeo, Loom etc.) não são assets.

## N005 — Publicação em staging e `--force` — Accepted

Substitui o `.part-*` + rename por arquivo da CLI Deno: o export inteiro é
gravado em `<out>/.notion-export-<slug>-<random>.tmp`, aberto como `os.Root`
filho (`safefs.Root.OpenRoot`), no mesmo filesystem do destino. A pasta só é
publicada por rename depois de todos os arquivos terem sido escritos e
sincronizados; erro/cancelamento a remove (somente se ainda for o mesmo objeto
criado, `os.SameFile`).

Sem `--force`, o rename só ocorre sobre um nome ausente. `rename(2)` não
substitui diretório não vazio e o Windows recusa destino existente, portanto
nenhum conteúdo existente pode ser perdido mesmo em corrida (um diretório
vazio criado nesse intervalo em Unix pode ser substituído, sem perda de dados).
A CLI Deno sobrescrevia arquivos do export anterior; agora isso exige `--force`.

`--force` só substitui um diretório real com `manifest.json` regular
(≤ 8 MiB) de `notion-export`/`notion-public-exporter` e o mesmo `pageId`.
O anterior é movido para `.notion-export-<slug>-<random>.old`, identidade
revalidada, staging instalado; falha na instalação restaura o anterior. A
quarentena só é apagada após sucesso e se ainda for o diretório validado;
caso contrário fica registrada como aviso. Testes injetam falhas de rename para
provar restauração.

`--out` é criado com `safefs.EnsureDir` somente quando o export começa a ser
gravado; o destino é inspecionado read-only antes de perguntas e downloads.

## N006 — Dependências — Accepted

Somente stdlib e `golang.org/x/text/unicode/norm` (NFKD para slugs e nomes
portáveis; a stdlib não normaliza Unicode). `x/text` 0.42.0 já estava fixado e
coberto por `THIRD_PARTY_NOTICES.txt` (M019 do media-get) e passa de indireto a
direto, sem nova versão nem nova transitiva.

## N007 — Plataformas — Accepted

Nenhum código por SO: `net/http`, `os.Root` (via `safefs`) e rename de
diretório confinado. Cross-build dos seis targets verificado localmente.
Runtime observado localmente em Linux amd64 (suíte e export real); macOS e
Windows ficam `untested` até a execução nativa do CI desta mudança.

## N008 — Reanálise de experiência e erros — Accepted (2026-11-12)

Aplicadas as lições dos ajustes pós-uso do media-get (progresso, texto remoto,
falhas só percebidas em uso real), antes da primeira publicação:

- `RetryPolicy.Notify` anuncia cada espera; o cooldown de 60 s após 429 deixa de
  parecer travamento;
- erros de transporte permanentes (verificação de certificado, redirect
  recusado pela política, cancelamento) não são repetidos;
- `Downloader.OnBytes` alimenta uma linha viva em TTY (contagem, bytes e total
  estimado, descontando transferências descartadas); sem TTY, linhas por evento;
- `--name` escolhe o diretório do export, validado como um único componente
  ASCII portável sem `.` inicial, resolvendo páginas homônimas;
- texto remoto exibido no terminal remove controles e caracteres de formato
  Unicode (bidi/zero-width);
- tamanhos em KiB/MiB, como no media-get;
- sobras de staging/quarentena de processos mortos são avisadas, nunca
  apagadas automaticamente.
