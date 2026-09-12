# repo_zip — entry point público.
#
# Cria snapshots ZIP de repositórios Git respeitando as regras reais de ignore.
# Os helpers ficam separados em common.nu, naming.nu, git.nu e archive.nu.
# Apenas `repo-zip` é exportado por este módulo.

use ../platform [require-runtime]
use ./common.nu [fail]
use ./naming.nu [normalize-base-name validate-suffix relative-if-inside]
use ./git.nu [
    resolve-repo-root
    git-head-label
    git-is-dirty
    git-state-token
    assert-no-submodules
    assert-no-sparse-checkout
    assert-git-dir-is-self-contained
    assert-no-git-locks
    tracked-output-guard
    list-archive-files
]
use ./archive.nu [add-files-to-zip add-git-dir-to-zip verify-zip]

const DEFAULT_OUTPUT_DIR = ".tmp/unzip"


# Cria um ZIP de um repositório Git.
#
# Exemplos:
#   repo-zip
#   repo-zip .
#   repo-zip minha-pasta -o aqui.zip
#   repo-zip . --name snapshot
#   repo-zip . -v v1.2.0
#   repo-zip . --git
#   repo-zip . --git -v v2
#   repo-zip . --git -o aqui.zip
#
# Sem -o, saída padrão:
#   <repo>/.tmp/unzip/<nome>.zip
#
# --git inclui .git e acrescenta <hash>[-dirty] somente ao nome automático.
# -o/--output é sempre respeitado exatamente como informado.
export def repo-zip [
    source?: path
    --output (-o): path
    --name (-n): string
    --git
    --suffix (-s): string
    --version (-v): string
    --force (-f)
] {
    require-runtime {
        name: "repo-zip"
        support: {
            macos: { status: "supported" }
            linux: {
                status: "unsupported"
                reason: "repo-zip ainda não foi implementado/testado no Linux"
            }
            windows: {
                status: "unsupported"
                reason: "repo-zip ainda não foi implementado/testado no Windows"
            }
            default: {
                status: "unsupported"
                reason: "sistema não reconhecido pelo config"
            }
        }
        requires: [
            { kind: "command", name: "git", hint: "instale o Git" }
            { kind: "path", path: "/usr/bin/zip", hint: "o macOS normalmente inclui /usr/bin/zip" }
            { kind: "command", name: "unzip", hint: "o macOS normalmente inclui /usr/bin/unzip" }
        ]
    }

    if ($output != null) and ($name != null) {
        fail "use --output ou --name, não os dois"
    }

    if ($suffix != null) and ($version != null) {
        fail "use --suffix ou --version, não os dois"
    }

    if ($output != null) and (($suffix != null) or ($version != null)) {
        fail "--output define o nome exato; coloque o sufixo diretamente no caminho passado a -o"
    }

    let source_arg = ($source | default ".")

    if not ($source_arg | path exists) {
        fail $"source não existe: ($source_arg)"
    }

    let source_abs = ($source_arg | path expand --strict)
    if (($source_abs | path type) != "dir") {
        fail $"source precisa ser um diretório: ($source_abs)"
    }

    let repo_root = (resolve-repo-root $source_abs)

    # Abortamos snapshots potencialmente incompletos em vez de omitir conteúdo
    # silenciosamente.
    assert-no-submodules $repo_root
    assert-no-sparse-checkout $repo_root

    let suffix_value = if $suffix != null {
        validate-suffix $suffix "--suffix"
    } else if $version != null {
        validate-suffix $version "--version"
    } else {
        null
    }

    let requested_base = if $name != null {
        normalize-base-name $name
    } else {
        $repo_root | path basename
    }

    if $git {
        assert-git-dir-is-self-contained $repo_root
        assert-no-git-locks $repo_root
    }

    # Se -o foi informado, conseguimos proteger/excluir esse path antes de
    # calcular o estado dirty. Sem -o, a saída fica em DEFAULT_OUTPUT_DIR,
    # diretório que já é excluído por inteiro da checagem.
    let explicit_output_path = if $output != null {
        let candidate = ($output | path expand)
        if not (($candidate | path basename | str lowercase) | str ends-with ".zip") {
            fail "--output precisa terminar em .zip"
        }
        $candidate
    } else {
        null
    }

    let explicit_output_rel = if $explicit_output_path != null {
        relative-if-inside $explicit_output_path $repo_root
    } else {
        null
    }

    tracked-output-guard $repo_root $explicit_output_rel

    let git_head = if $git {
        git-head-label $repo_root
    } else {
        null
    }

    let dirty = if $git {
        git-is-dirty $repo_root $DEFAULT_OUTPUT_DIR $explicit_output_rel
    } else {
        false
    }

    let initial_git_state = if $git {
        git-state-token $repo_root $DEFAULT_OUTPUT_DIR [$explicit_output_rel]
    } else {
        null
    }

    let auto_name_parts = (
        [
            $requested_base
            $git_head
            (if $git and $dirty { "dirty" } else { null })
            $suffix_value
        ]
        | compact
    )
    let auto_filename = $"($auto_name_parts | str join "-").zip"

    let raw_output = if $explicit_output_path != null {
        $explicit_output_path
    } else {
        $repo_root | path join $DEFAULT_OUTPUT_DIR $auto_filename
    }

    let output_parent_unresolved = ($raw_output | path dirname)
    mkdir $output_parent_unresolved
    let output_parent = ($output_parent_unresolved | path expand --strict)
    let output_path = ($output_parent | path join ($raw_output | path basename))
    let output_rel = (relative-if-inside $output_path $repo_root)

    # Reaplica depois da resolução final do destino.
    tracked-output-guard $repo_root $output_rel

    let output_type = ($output_path | path type | default null)
    if $output_type != null {
        if $output_type != "file" {
            fail $"o destino já existe e não é um arquivo comum: ($output_path)"
        }
        if not $force {
            fail $"o destino já existe: ($output_path) — use --force para substituir"
        }
    }

    let files = (list-archive-files $repo_root $DEFAULT_OUTPUT_DIR $output_rel)
    if (($files | is-empty) and (not $git)) {
        fail "nenhum arquivo elegível para compactar"
    }

    # O temporário fica no mesmo filesystem/diretório do destino para que a
    # publicação final seja um rename, não uma cópia parcial do ZIP.
    let temp_name = $".($output_path | path basename).repo-zip-(random uuid).tmp.zip"
    let temp_zip = ($output_parent | path join $temp_name)
    let temp_rel = (relative-if-inside $temp_zip $repo_root)

    if ($temp_zip | path exists) {
        fail "colisão inesperada ao criar o arquivo temporário"
    }

    try {
        if ($files | is-not-empty) {
            add-files-to-zip $repo_root $temp_zip $files
        }

        if $git {
            # Uma operação Git pode ter começado depois da checagem inicial.
            assert-no-git-locks $repo_root
            add-git-dir-to-zip $repo_root $temp_zip
            assert-no-git-locks $repo_root
        }

        if not ($temp_zip | path exists) {
            fail "o zip não produziu um arquivo de saída"
        }

        verify-zip $temp_zip

        if $git {
            let final_git_state = (
                git-state-token $repo_root $DEFAULT_OUTPUT_DIR [$output_rel $temp_rel]
            )
            if $final_git_state != $initial_git_state {
                fail "o estado do repositório mudou durante o snapshot --git; ZIP temporário descartado para evitar backup inconsistente"
            }
        }

        # Defesa em profundidade imediatamente antes da publicação.
        tracked-output-guard $repo_root $output_rel

        let final_type = ($output_path | path type | default null)
        if $final_type != null {
            if $final_type != "file" {
                fail $"o destino mudou durante a execução e deixou de ser um arquivo comum: ($output_path)"
            }
            if not $force {
                fail $"o destino apareceu durante a execução: ($output_path)"
            }
        }

        if $force {
            mv --force $temp_zip $output_path
        } else {
            mv --no-clobber $temp_zip $output_path
            if ($temp_zip | path exists) {
                fail $"o destino apareceu durante a publicação: ($output_path)"
            }
        }

        if (($output_path | path type | default null) != "file") {
            fail "falha ao publicar o ZIP final"
        }
    } finally {
        # Único rm do módulo: exclusivamente o temporário criado nesta execução.
        if (
            ($temp_zip | path exists)
            and (($temp_zip | path dirname) == $output_parent)
        ) {
            rm --permanent --force $temp_zip
        }
    }

    $output_path
}
