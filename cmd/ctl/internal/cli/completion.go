package cli

import (
	"fmt"
	"os"
)

func CmdCompletion(shell string) error {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "지원하지 않는 쉘: %s (bash | zsh | fish)\n", shell)
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
}

const bashCompletion = `# mboy bash completion
# 설치: source <(mboy completion bash)
# 영구 설치: mboy completion bash > /etc/bash_completion.d/mboy

_mboy_completion() {
    local cur prev words cword
    _init_completion || return

    local commands="login status user repo member package build version completion"
    local user_cmds="list create update delete"
    local repo_cmds="list create get update delete"
    local member_cmds="list add update remove"
    local package_cmds="search"
    local build_cmds="list trigger get"

    case "${words[1]}" in
        user)    COMPREPLY=($(compgen -W "$user_cmds"    -- "$cur")) ; return ;;
        repo)    COMPREPLY=($(compgen -W "$repo_cmds"    -- "$cur")) ; return ;;
        member)  COMPREPLY=($(compgen -W "$member_cmds"  -- "$cur")) ; return ;;
        package) COMPREPLY=($(compgen -W "$package_cmds" -- "$cur")) ; return ;;
        build)   COMPREPLY=($(compgen -W "$build_cmds"   -- "$cur")) ; return ;;
        completion) COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur")) ; return ;;
    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "--server --token --json --version" -- "$cur"))
        return
    fi

    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
}

complete -F _mboy_completion mboy
`

const zshCompletion = `#compdef mboy
# mboy zsh completion
# 설치: source <(mboy completion zsh)
# 영구 설치: mboy completion zsh > "${fpath[1]}/_mboy"

_mboy() {
    local -a commands
    commands=(
        'login:서버 인증'
        'status:서버 상태 확인'
        'user:유저 관리'
        'repo:리포지토리 관리'
        'member:멤버 관리'
        'package:패키지 검색'
        'build:빌드 관리'
        'completion:쉘 자동완성 스크립트 출력'
        'version:버전 출력'
    )

    local -a global_opts
    global_opts=(
        '--server[서버 URL]:url'
        '--token[API 토큰]:token'
        '--json[JSON 출력]'
        '--version[버전 출력]'
    )

    _arguments -C $global_opts \
        '1:command:->cmd' \
        '2:subcommand:->sub' && return

    case $state in
        cmd) _describe 'command' commands ;;
        sub)
            case $words[2] in
                user)    local -a s=(list create update delete)     ; _describe 'subcommand' s ;;
                repo)    local -a s=(list create get update delete) ; _describe 'subcommand' s ;;
                member)  local -a s=(list add update remove)        ; _describe 'subcommand' s ;;
                package) local -a s=(search)                        ; _describe 'subcommand' s ;;
                build)   local -a s=(list trigger get)              ; _describe 'subcommand' s ;;
                completion) local -a s=(bash zsh fish)              ; _describe 'shell' s ;;
            esac
        ;;
    esac
}

_mboy "$@"
`

const fishCompletion = `# mboy fish completion
# 설치: mboy completion fish | source
# 영구 설치: mboy completion fish > ~/.config/fish/completions/mboy.fish

set -l commands login status user repo member package build completion version

complete -c mboy -f
complete -c mboy -n "__fish_use_subcommand" -a login      -d '서버 인증'
complete -c mboy -n "__fish_use_subcommand" -a status     -d '서버 상태'
complete -c mboy -n "__fish_use_subcommand" -a user       -d '유저 관리'
complete -c mboy -n "__fish_use_subcommand" -a repo       -d '리포지토리 관리'
complete -c mboy -n "__fish_use_subcommand" -a member     -d '멤버 관리'
complete -c mboy -n "__fish_use_subcommand" -a package    -d '패키지 검색'
complete -c mboy -n "__fish_use_subcommand" -a build      -d '빌드 관리'
complete -c mboy -n "__fish_use_subcommand" -a completion -d '자동완성 스크립트'
complete -c mboy -n "__fish_use_subcommand" -a version    -d '버전 출력'

# user subcommands
complete -c mboy -n "__fish_seen_subcommand_from user" -a "list create update delete" -f
# repo subcommands
complete -c mboy -n "__fish_seen_subcommand_from repo" -a "list create get update delete" -f
# member subcommands
complete -c mboy -n "__fish_seen_subcommand_from member" -a "list add update remove" -f
# package subcommands
complete -c mboy -n "__fish_seen_subcommand_from package" -a "search" -f
# build subcommands
complete -c mboy -n "__fish_seen_subcommand_from build" -a "list trigger get" -f
# completion shells
complete -c mboy -n "__fish_seen_subcommand_from completion" -a "bash zsh fish" -f

# global flags
complete -c mboy -l server  -d '서버 URL'
complete -c mboy -l token   -d 'API 토큰'
complete -c mboy -l json    -d 'JSON 출력'
complete -c mboy -l version -d '버전 출력'
`
