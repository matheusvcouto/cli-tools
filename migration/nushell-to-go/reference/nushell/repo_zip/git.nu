# Operações Git e proteções específicas do repositório.

use ./common.nu [fail external-detail]


def git-result-or-error [result: record action: string] {
    if $result.exit_code != 0 {
        let detail = (external-detail $result)
        if ($detail | is-empty) {
            fail $action
        } else {
            fail $"($action): ($detail)"
        }
    }

    $result
}


export def resolve-repo-root [source_abs: string] {
    let result = (do {
        ^git -C $source_abs rev-parse --show-toplevel
    } | complete)

    git-result-or-error $result "source não está dentro de um repositório Git válido" | ignore

    let root_text = ($result.stdout | str trim --right --char (char nl))
    $root_text | path expand --strict
}


export def git-head-label [repo_root: string] {
    let head = (do {
        ^git -C $repo_root rev-parse --verify --short=12 HEAD
    } | complete)

    if $head.exit_code == 0 {
        return ($head.stdout | str trim)
    }

    # Repositório recém-criado sem commit.
    let symbolic = (do {
        ^git -C $repo_root symbolic-ref -q HEAD
    } | complete)

    if $symbolic.exit_code == 0 {
        "no-commit"
    } else {
        let detail = (external-detail $head)
        if ($detail | is-empty) {
            fail "não foi possível resolver HEAD"
        } else {
            fail $"não foi possível resolver HEAD: ($detail)"
        }
    }
}


def status-result [repo_root: string default_output_dir: string excluded_rels: list] {
    let default_output_exclude = (":(exclude)" + $default_output_dir + "/**")
    let explicit_excludes = (
        $excluded_rels
        | compact
        | each {|rel| ":(exclude,literal)" + $rel }
    )
    let pathspecs = ["." $default_output_exclude ...$explicit_excludes]

    let result = (do {
        ^git -C $repo_root status --porcelain=v1 -z --untracked-files=all -- ...$pathspecs
    } | complete)

    git-result-or-error $result "não foi possível verificar o estado do repositório"
}


export def git-is-dirty [repo_root: string default_output_dir: string output_rel] {
    let status = (status-result $repo_root $default_output_dir [$output_rel])
    not ($status.stdout | is-empty)
}


# Token leve para detectar alterações Git enquanto um snapshot --git está
# sendo produzido. Não torna o filesystem atomic, mas impede publicar o ZIP
# com hash/estado sabidamente desatualizados quando HEAD/status mudam.
export def git-state-token [repo_root: string default_output_dir: string excluded_rels: list] {
    let head = (git-head-label $repo_root)
    let status = (status-result $repo_root $default_output_dir $excluded_rels)

    {
        head: $head
        status: $status.stdout
    }
}


export def assert-no-submodules [repo_root: string] {
    let staged = (do {
        ^git -C $repo_root ls-files --stage
    } | complete)

    git-result-or-error $staged "não foi possível inspecionar o índice Git" | ignore

    let has_submodule = (
        $staged.stdout
        | lines
        | any {|line| $line | str starts-with "160000 " }
    )

    if $has_submodule {
        fail "submodules foram detectados; suporte seguro a submodules ainda não foi implementado"
    }
}


# Sparse checkout/skip-worktree faria arquivos tracked ausentes do working tree
# desaparecerem do snapshot. É mais seguro abortar do que produzir ZIP parcial.
export def assert-no-sparse-checkout [repo_root: string] {
    let listed = (do {
        ^git -C $repo_root ls-files -t
    } | complete)

    git-result-or-error $listed "não foi possível verificar sparse checkout" | ignore

    let has_skip_worktree = (
        $listed.stdout
        | lines
        | any {|line| $line | str starts-with "S " }
    )

    if $has_skip_worktree {
        fail "sparse checkout/skip-worktree detectado; recusando gerar um snapshot silenciosamente incompleto"
    }
}


export def assert-git-dir-is-self-contained [repo_root: string] {
    let dot_git = ($repo_root | path join ".git")
    let git_type = ($dot_git | path type)

    if $git_type == "file" {
        fail "--git não suporta worktrees/submodules com .git como arquivo; recusando gerar um backup incompleto"
    }

    if $git_type != "dir" {
        fail "--git exige um diretório .git normal dentro da raiz do repositório"
    }

    let env_alternates = ($env | get --optional GIT_ALTERNATE_OBJECT_DIRECTORIES | default "" | str trim)
    if ($env_alternates | is-not-empty) {
        fail "--git detectou GIT_ALTERNATE_OBJECT_DIRECTORIES; o repositório pode depender de objetos externos"
    }

    let alternates = ($dot_git | path join "objects" "info" "alternates")
    if (($alternates | path type) == "file") {
        let alternates_text = (open --raw $alternates | decode utf-8 | str trim)

        if ($alternates_text | is-not-empty) {
            fail "--git detectou .git/objects/info/alternates; o repositório depende de objetos externos e o ZIP não seria autocontido"
        }
    }
}


# Locks conhecidos indicam uma escrita Git em andamento ou interrompida. Não
# tocamos neles; apenas recusamos --git para não copiar metadados inconsistentes.
export def assert-no-git-locks [repo_root: string] {
    let dot_git = ($repo_root | path join ".git")
    let known_locks = [
        ($dot_git | path join "index.lock")
        ($dot_git | path join "HEAD.lock")
        ($dot_git | path join "config.lock")
        ($dot_git | path join "packed-refs.lock")
        ($dot_git | path join "shallow.lock")
    ]

    let present = ($known_locks | where {|path| $path | path exists })
    if ($present | is-not-empty) {
        fail $"--git detectou lock do Git: ($present.0)"
    }
}


export def tracked-output-guard [repo_root: string output_rel] {
    if $output_rel == null {
        return
    }

    if ($output_rel == ".git") or ($output_rel | str starts-with ".git/") {
        fail "o arquivo de saída não pode ficar dentro de .git"
    }

    let tracked = (do {
        ^git --literal-pathspecs -C $repo_root ls-files --error-unmatch -- $output_rel
    } | complete)

    if $tracked.exit_code == 0 {
        fail $"o destino ($output_rel) é tracked pelo Git; recusando sobrescrever um arquivo do repositório"
    }

    # 1 = path não encontrado. Outros códigos são falha real.
    if $tracked.exit_code != 1 {
        let detail = (external-detail $tracked)
        if ($detail | is-empty) {
            fail "não foi possível verificar se o destino é tracked"
        } else {
            fail $"não foi possível verificar se o destino é tracked: ($detail)"
        }
    }
}


export def list-archive-files [repo_root: string default_output_dir: string output_rel] {
    let listed = (do {
        ^git -C $repo_root ls-files -z --cached --others --exclude-standard
    } | complete)

    git-result-or-error $listed "não foi possível listar os arquivos do repositório" | ignore

    let raw = ($listed.stdout | into binary)
    let files = (
        $raw
        | bytes split (char nul)
        | each {|chunk|
            if (($chunk | bytes length) == 0) {
                null
            } else {
                $chunk | decode utf-8
            }
        }
        | compact
        | uniq
    )

    $files
    | where {|rel|
        let in_internal_output = (
            ($rel == $default_output_dir)
            or ($rel | str starts-with $"($default_output_dir)/")
        )
        let is_requested_output = ($output_rel != null) and ($rel == $output_rel)
        let full = ($repo_root | path join $rel)
        let kind = ($full | path type | default null)

        # Arquivos tracked removidos ainda podem constar no índice.
        (
            (not $in_internal_output)
                and (not $is_requested_output)
                and ($kind in ["file" "symlink"])
        )
    }
}
