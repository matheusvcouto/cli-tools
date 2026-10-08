# Tarefas

Plano de investigação para entrada JSON e execução concorrente do lote:
[plans/media-get/BATCH_JSON.md](plans/media-get/BATCH_JSON.md).
Implementação local em revisão; publicação depende da aprovação do usuário.

- [x] **media-get:** formato/extensão explícitos no resumo, nome previsto com extensão quando conhecida e aviso de extensão a confirmar no automático. Colisões recebem sufixo; o nome previsto não é uma reserva.

- [x] **media-get:** lote por manifesto JSON nativo ou export `videos`, com nomes, opções comuns/por item, revisão/edição/exclusão, concorrência limitada, falhas parciais e cancelamento. Schema/exemplo/validação offline disponíveis; implementação local para teste, ainda não publicada.

- [x] **media-get:** edição direta do formato de saída, aplicação de MP4 ao lote, indicadores nas consultas e painel de progresso com barra geral e barras por download ativo. Validado localmente; aguarda teste do usuário.

- [ ] **media-get — próxima versão:** permitir selecionar múltiplas saídas no menu de download com a barra de espaço, navegar com as setas e confirmar o conjunto com Enter. Exemplos: vídeo + áudio, vídeo + legenda ou vídeo + áudio + legenda. Após a seleção, configurar os formatos e demais opções de cada saída, como qualidade do vídeo e idioma/formato da legenda, e mostrar todas as saídas no resumo final. Executar o conjunto a partir da mesma URL, sem exigir que o usuário repita o fluxo para cada arquivo. Manter a seleção de uma única saída disponível e definir/testar cancelamento e falhas parciais do conjunto antes da publicação.

- [ ] **media-get:** entrada adicional de lote como lista de URLs por linhas, reutilizando o planejador/fila já implementados para JSON. Avaliar um separador explícito como vírgula sem quebrar URLs que contenham esse caractere. Oferecer opções comuns interativas (tipo/formato/qualidade/destino), títulos originais ou nomes individuais e exclusão de itens na revisão. Manter os testes de cancelamento, falhas e colisões do lote.
