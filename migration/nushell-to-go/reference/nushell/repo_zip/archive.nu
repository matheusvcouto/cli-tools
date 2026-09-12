# Criação e validação do arquivo ZIP.

use ./common.nu [fail external-detail]

const ZIP_BATCH_SIZE = 64


export def add-files-to-zip [repo_root: string temp_zip: string files: list<string>] {
    for batch in ($files | chunks $ZIP_BATCH_SIZE) {
        # ./ impede nomes como -foo de serem interpretados como flags do zip.
        let zip_paths = ($batch | each {|rel| $"./($rel)" })
        let result = (do {
            cd $repo_root
            ^zip -q -y $temp_zip ...$zip_paths
        } | complete)

        if $result.exit_code != 0 {
            let detail = (external-detail $result)
            if ($detail | is-empty) {
                fail "o comando zip falhou ao adicionar arquivos"
            } else {
                fail $"o comando zip falhou ao adicionar arquivos: ($detail)"
            }
        }
    }
}


export def add-git-dir-to-zip [repo_root: string temp_zip: string] {
    let result = (do {
        cd $repo_root
        ^zip -q -y -r $temp_zip "./.git"
    } | complete)

    if $result.exit_code != 0 {
        let detail = (external-detail $result)
        if ($detail | is-empty) {
            fail "o comando zip falhou ao adicionar .git"
        } else {
            fail $"o comando zip falhou ao adicionar .git: ($detail)"
        }
    }
}


export def verify-zip [temp_zip: string] {
    let result = (do {
        ^unzip -tqq $temp_zip
    } | complete)

    if $result.exit_code != 0 {
        let detail = (external-detail $result)
        if ($detail | is-empty) {
            fail "o ZIP criado falhou na verificação de integridade"
        } else {
            fail $"o ZIP criado falhou na verificação de integridade: ($detail)"
        }
    }
}
