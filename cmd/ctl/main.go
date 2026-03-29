// mboy — command-line management tool for miraeboy
//
// Usage:
//
//	mboy [--server URL] [--token TOKEN] <command> [args...]
//
// Commands:
//
//	login                          서버 인증 (토큰 저장)
//	status                         서버 상태 확인
//	repo list                      리포지토리 목록
//	repo create                    리포지토리 생성
//	repo get    <name>             리포지토리 정보
//	repo update <name>             리포지토리 수정
//	repo delete <name>             리포지토리 삭제
//	member list   <repo>           멤버 목록
//	member add    <repo> <user> <perm>  멤버 추가
//	member update <repo> <user> <perm>  멤버 권한 수정
//	member remove <repo> <user>    멤버 제거
//	package search <repo> [query]  패키지 검색
//	build list                     빌드 목록
//	build trigger                  빌드 트리거
//	build get   <id>               빌드 상태 조회
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/corbie79/miraeboy/cmd/ctl/internal/cli"
)

var version = "dev"

func main() {
	// ── 전역 플래그 ──────────────────────────────────────────────────────────
	fs := flag.NewFlagSet("mboy", flag.ContinueOnError)
	serverURL := fs.String("server", "", "miraeboy 서버 URL (기본값: 저장된 설정 또는 http://localhost:9300)")
	token := fs.String("token", "", "Bearer 토큰 (기본값: 저장된 토큰)")
	outputJSON := fs.Bool("json", false, "JSON 형식으로 출력")
	showVersion := fs.Bool("version", false, "버전 출력")

	fs.Usage = usage
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if *showVersion {
		fmt.Println(version)
		return
	}

	args := fs.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	// ── 설정 로드 / 전역 플래그로 오버라이드 ─────────────────────────────────
	cfg, err := cli.LoadConfig()
	if err != nil {
		die("config 로드 실패: %v", err)
	}
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *token != "" {
		cfg.Token = *token
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:9300"
	}

	client := cli.NewClient(cfg)
	printer := cli.NewPrinter(*outputJSON)

	// ── 커맨드 라우팅 ─────────────────────────────────────────────────────────
	cmd, sub := args[0], ""
	rest := args[1:]
	if len(rest) > 0 && !isFlag(rest[0]) {
		sub = rest[0]
		rest = rest[1:]
	}

	var runErr error

	switch cmd {
	case "login":
		runErr = cli.CmdLogin(client, cfg, rest)

	case "status":
		runErr = cli.CmdStatus(client, printer)

	case "repo":
		switch sub {
		case "list":
			runErr = cli.CmdRepoList(client, printer, rest)
		case "create":
			runErr = cli.CmdRepoCreate(client, printer, rest)
		case "get":
			runErr = cli.CmdRepoGet(client, printer, rest)
		case "update":
			runErr = cli.CmdRepoUpdate(client, printer, rest)
		case "delete":
			runErr = cli.CmdRepoDelete(client, printer, rest)
		default:
			fmt.Fprintf(os.Stderr, "Unknown repo subcommand: %q\n", sub)
			repoUsage()
			os.Exit(1)
		}

	case "member":
		switch sub {
		case "list":
			runErr = cli.CmdMemberList(client, printer, rest)
		case "add":
			runErr = cli.CmdMemberAdd(client, printer, rest)
		case "update":
			runErr = cli.CmdMemberUpdate(client, printer, rest)
		case "remove":
			runErr = cli.CmdMemberRemove(client, printer, rest)
		default:
			fmt.Fprintf(os.Stderr, "Unknown member subcommand: %q\n", sub)
			memberUsage()
			os.Exit(1)
		}

	case "package":
		switch sub {
		case "search":
			runErr = cli.CmdPackageSearch(client, printer, rest)
		default:
			fmt.Fprintf(os.Stderr, "Unknown package subcommand: %q\n", sub)
			os.Exit(1)
		}

	case "build":
		switch sub {
		case "list":
			runErr = cli.CmdBuildList(client, printer, rest)
		case "trigger":
			runErr = cli.CmdBuildTrigger(client, printer, rest)
		case "get":
			runErr = cli.CmdBuildGet(client, printer, rest)
		default:
			fmt.Fprintf(os.Stderr, "Unknown build subcommand: %q\n", sub)
			os.Exit(1)
		}

	case "version":
		fmt.Println(version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		usage()
		os.Exit(1)
	}

	if runErr != nil {
		// API 오류는 JSON 형식일 수 있음 — 가능하면 에러 메시지만 출력
		var apiErr struct{ Error string }
		if json.Unmarshal([]byte(runErr.Error()), &apiErr) == nil && apiErr.Error != "" {
			fmt.Fprintln(os.Stderr, "Error:", apiErr.Error)
		} else {
			fmt.Fprintln(os.Stderr, "Error:", runErr)
		}
		os.Exit(1)
	}
}

func isFlag(s string) bool { return len(s) > 0 && s[0] == '-' }

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Print(`mboy — miraeboy 서버 관리 CLI

사용법:
  mboy [--server URL] [--token TOKEN] [--json] <command> [subcommand] [args]

전역 옵션:
  --server URL   서버 주소 (기본값: http://localhost:9300)
  --token TOKEN  API 토큰 (login 후 자동 저장)
  --json         JSON 형식으로 출력
  --version      버전 출력

커맨드:
  login                            서버에 로그인 (토큰 저장)
  status                           서버 상태 확인

  repo list                        리포지토리 목록
  repo create --name NAME ...      리포지토리 생성
  repo get    <name>               리포지토리 상세
  repo update <name> [옵션...]     리포지토리 수정
  repo delete <name> [--force]     리포지토리 삭제

  member list   <repo>             멤버 목록
  member add    <repo> <user> <perm>  멤버 추가/수정
  member update <repo> <user> <perm>  멤버 권한 변경
  member remove <repo> <user>      멤버 제거

  package search <repo> [query]    패키지 검색

  build list                       빌드 목록
  build trigger --repo NAME        빌드 트리거
  build get <id>                   빌드 상태 조회

예시:
  mboy login --user admin --password secret
  mboy repo list
  mboy repo create --name mylib --owner admin
  mboy member add mylib alice write
  mboy package search mylib "boost*"
`)
}

func repoUsage() {
	fmt.Print(`
repo 서브커맨드: list | create | get | update | delete
`)
}

func memberUsage() {
	fmt.Print(`
member 서브커맨드: list | add | update | remove
`)
}
