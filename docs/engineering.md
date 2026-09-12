# Engenharia

## Princípios

1. evidência antes de conclusão;
2. simplicidade antes de abstração;
3. segurança antes de “zero dependências”;
4. comportamento observável antes de equivalência presumida;
5. mudanças pequenas, testáveis e rastreáveis.

## Evidência

Classifique afirmações relevantes como:

- **observado**: código/doc oficial/teste comprova;
- **inferido**: razoável, ainda não comprovado;
- **não verificado**: exige ambiente/capacidade fora da suíte segura.

Não promover inferência a suporte confirmado.

## Erros Go

- preservar causa com `%w`;
- usar `errors.Is/As` quando apropriado;
- erro deve incluir contexto operacional sem vazar segredo;
- não depender de comparação de texto de erro para lógica.

## Interfaces

Não criar interface apenas para “boas práticas”. Use-a quando houver fronteira real para teste, plataforma ou múltiplas implementações.

Clock/random/runner podem ser dependências explícitas quando determinismo exige; não transforme todo pacote em DI framework.

## Plataforma

Regra de negócio não seleciona SO. Quando houver diferença real de implementação, use uma capability estreita ou arquivo por build tag conforme `docs/platforms.md`; não crie interface apenas para esconder uma chamada de stdlib. `unsupported` é melhor que fallback inseguro.

## Paths/filesystem

- `filepath`, não manipulação manual de separador;
- path absoluto/limpo não é identidade canônica: para dois objetos existentes
  que devem ser o mesmo, usar `os.Stat` + `os.SameFile`; preservar `Lstat` quando
  seguir o symlink mudaria a política;
- igualdade textual só quando a grafia for parte do contrato; `/var` e
  `/private/var` são o caso clássico de alias observado no macOS;
- containment nunca por `strings.HasPrefix`;
- `Lstat`/`Readlink` quando symlink importa;
- `os.Root` quando uma operação precisa ficar confinada;
- temporário criado com nome imprevisível e create exclusivo;
- não usar remove-then-rename para substituir arquivo importante;
- não alegar atomicidade/durabilidade além do que a plataforma garante.

`os.Root` restringe resolução dentro de uma raiz, mas `os.OpenRoot` pode seguir symlink no path usado para abrir a própria raiz. A camada `safefs` deve fixar/revalidar a identidade dessa raiz e ainda decidir deliberadamente se symlinks internos devem ser seguidos, arquivados ou rejeitados.

## Estado persistente

- schema versionado;
- decode valida estrutura e invariantes;
- corrupção nunca vira estado vazio;
- read-modify-write protegido contra concorrência;
- commit seguro por plataforma;
- backup é recuperação, não substituto de commit correto;
- estado legado nunca é apagado automaticamente na migração.

## Processos

- `exec.Command`/equivalente com argv separado;
- sem shell para construir comandos;
- env filho construído explicitamente quando há isolamento;
- não registrar valores de secrets;
- preservar exit status conforme contrato;
- ACP não escreve nada próprio em stdout.

## Dependências

Não há meta de “zero deps” a qualquer custo. Há meta de **poucas deps justificadas**.

Antes de adicionar uma:

- existe lacuna real?
- a implementação caseira seria mais frágil?
- o projeto é mantido/licença aceitável?
- quantas transitivas entram?
- temos teste que prova a semântica necessária?

Frameworks de CLI/config não entram enquanto o parser/config continuarem pequenos.

## Toolchain

- `mise.toml` fixa a toolchain usada pelo repo;
- não executar `go env -w` em nome do usuário;
- `GOTOOLCHAIN=local` no ambiente do repo evita download implícito de toolchain;
- downloads de dependência pertencem ao bootstrap/CI, não ao comportamento dos testes.

## Documentação

Documentação ativa descreve o estado atual. Planos/migrações concluídos vão para `docs/history/` e deixam de ser leitura obrigatória.
