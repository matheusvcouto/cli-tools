# notion-export

Exporta uma página **pública** do Notion (compartilhada com "Compartilhar na
web") para Markdown, HTML estático e assets locais. Não usa token, cookie nem a
API oficial: lê a mesma API interna usada pelo cliente web do Notion para
páginas públicas, como o ecossistema `notion-client`/`react-notion-x`.

> Ferramenta pessoal de arquivamento, sem afiliação com a Notion Labs, Inc.
> A API interna não é documentada e pode mudar sem aviso. Exporte apenas
> conteúdo que você tem direito de arquivar.

Estado: **experimental**. Sucessor em Go da CLI Deno `notion-public-exporter`.

## Uso

```sh
notion-export "https://example.notion.site/Minha-Pagina-0123456789abcdef0123456789abcdef"
notion-export 0123456789abcdef0123456789abcdef --yes
notion-export 01234567-89ab-cdef-0123-456789abcdef --format html --out ./backup
notion-export --from-json exports/minha-pagina/page.json --out ./rebuilt
```

A entrada aceita URL pública (`notion.so`, `notion.site`, `app.notion.com/p/...`),
UUID ou page ID compacto. A query e o fragmento da URL nunca fornecem o ID.

Progresso, perguntas e o resumo vão para stderr; stdout recebe apenas o caminho
do diretório exportado, para uso em scripts. Em terminal, os downloads mostram
uma linha viva com quantidade, bytes recebidos e total estimado; com stderr
redirecionado, uma linha por evento. Esperas longas (por exemplo o intervalo de
60 s após `429` do Notion) são anunciadas com o motivo e a duração.

O diretório é `<out>/<slug-do-título>`. Para páginas com o mesmo título, ou
para escolher outro nome, use `--name` (letras ASCII, números, `.`, `_` e `-`,
sem começar com `.`).

## Saída

```text
exports/
└── minha-pagina/
    ├── page.md
    ├── page.html
    ├── styles.css
    ├── page.json        # recordMap usado (inclui signed URLs temporárias)
    ├── manifest.json    # metadados e status de cada asset
    └── assets/{images,videos,audio,pdfs,files}/
```

- `page.md`: CommonMark/GFM. O texto do Notion é escapado e aparece literal
  (inclusive HTML bruto); o título da página é o único `#`, e os títulos do
  Notion começam em `##`.
- `page.html`: sem JavaScript, CSS local, CSP restritiva e
  `referrer=no-referrer`. Embeds externos viram links, nunca iframes;
  vídeo/áudio locais usam elementos HTML5 nativos.
- `manifest.json`: `schemaVersion` 1, mesmo layout camelCase da CLI Deno.

Blocos suportados: texto e rich text (negrito, itálico, tachado, sublinhado,
código, links, menções de página/data, equações inline), títulos, listas
(aninhadas, numeradas, to-do), toggle, citação, callout, divisor, código,
equação, tabela simples, colunas, synced blocks, link para página, subpáginas
(como link público), imagens, vídeo, áudio, PDF, arquivos, bookmarks e embeds.
Bases de dados (collection views) não são exportadas e aparecem como aviso.

## Assets

Antes de baixar, a CLI estima o tamanho com uma requisição `GET` de 1 byte
(`Range: bytes=0-0`; o host de arquivos do Notion recusa `HEAD`) e pergunta por
tipo:

```text
Assets encontrados:
  Imagens: 9 (14.8 MiB)
  Vídeos: 2 (126.3 MiB)
Encontradas 9 imagens (14.8 MiB). Baixar imagens? [s/N]
```

| Flag | Efeito |
| --- | --- |
| `-y`, `--yes` | baixa todos os tipos |
| `--images`, `--videos`, `--audio`, `--pdfs`, `--attachments` | `ask`, `yes` ou `no`; prevalecem sobre `--yes` |
| `--no-size-probe` | não faz as requisições de estimativa |
| `--concurrency <n>` | requisições/downloads paralelos, 1 a 64 (padrão 6) |

A resposta vazia é **não**. Sem terminal em stdin, `ask` vale como `no`, com
um aviso. Vídeos de YouTube/Vimeo e similares permanecem links.

Cada download valida `Content-Type` contra o tipo do asset (por exemplo, uma
página HTML de erro não vira `.mp4`), confere `Content-Length`, aborta após 60 s
sem dados e repete falhas transitórias. Erros permanentes (certificado TLS
inválido, redirect recusado, 4xx) não são repetidos. A falha de um asset não interrompe os
demais: a saída mantém a URL remota segura e o manifesto registra o erro.

## Publicação e `--force`

O export inteiro é escrito numa pasta temporária oculta no próprio `--out`
(`.notion-export-*.tmp`) e só é movido para `<out>/<slug>` depois de todos os
arquivos terem sido gravados. Falha ou Ctrl+C removem essa pasta e não tocam o
destino.

Sem `--force`, um destino existente é recusado antes de qualquer pergunta ou
download. Com `--force`, o destino só é substituído se for um diretório real
com `manifest.json` gerado por `notion-export` (ou pela CLI Deno) **para a mesma
página**; caso contrário nada é alterado. O export anterior vai para uma
quarentena, é restaurado se a instalação falhar e só é apagado depois do
sucesso.

`--out` (padrão `./exports`) é criado quando necessário, sem seguir symlinks.
Se um processo anterior foi morto antes da limpeza, a sobra
(`.notion-export-<slug>-*.tmp`/`.old`) é apenas avisada: nada é apagado sem
prova de posse.

## Reprocessar `page.json`

`--from-json` reconstrói Markdown/HTML de um `page.json` salvo, sem consultar a
API do Notion. A página exportada é a indicada pelo `manifest.json` ao lado do
arquivo, quando existir. As signed URLs expiram: se os downloads falharem,
exporte novamente pela URL pública.

## Plataformas

Implementação portável (stdlib + `os.Root`), sem código por SO. Ver
[docs/platforms.md](../../docs/platforms.md) para o estado de evidência.
Decisões: [ADR](ADR.md).
