package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
)

// ─── repo list ────────────────────────────────────────────────────────────────

func CmdRepoList(client *Client, p *Printer, args []string) error {
	data, err := client.Get("/api/repos")
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Repositories []struct {
			Name            string `json:"name"`
			Owner           string `json:"owner"`
			Description     string `json:"description"`
			AnonymousAccess string `json:"anonymous_access"`
			MemberCount     int    `json:"member_count"`
			GitURL          string `json:"git_url"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if len(resp.Repositories) == 0 {
		fmt.Println("(리포지토리 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Repositories))
	for i, r := range resp.Repositories {
		git := ""
		if r.GitURL != "" {
			git = "✓"
		}
		rows[i] = []string{
			r.Name, r.Owner,
			r.AnonymousAccess,
			fmt.Sprintf("%d", r.MemberCount),
			git,
			r.Description,
		}
	}
	p.Table([]string{"NAME", "OWNER", "ANON", "MEMBERS", "GIT", "DESCRIPTION"}, rows)
	return nil
}

// ─── repo create ──────────────────────────────────────────────────────────────

func CmdRepoCreate(client *Client, p *Printer, args []string) error {
	fs := flag.NewFlagSet("repo create", flag.ContinueOnError)
	name := fs.String("name", "", "리포지토리 이름 (필수)")
	owner := fs.String("owner", "", "소유자 (필수)")
	desc := fs.String("description", "", "설명")
	anon := fs.String("anonymous", "none", "익명 접근: none|read|write")
	ns := fs.String("namespaces", "", "허용 네임스페이스 (쉼표 구분)")
	ch := fs.String("channels", "", "허용 채널 (쉼표 구분)")
	gitURL := fs.String("git-url", "", "git 연동 URL")
	gitBranch := fs.String("git-branch", "main", "git 브랜치")
	gitToken := fs.String("git-token", "", "git 인증 토큰")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name 이 필요합니다")
	}
	if *owner == "" {
		return fmt.Errorf("--owner 가 필요합니다")
	}

	body := map[string]any{
		"name":             *name,
		"owner":            *owner,
		"description":      *desc,
		"anonymous_access": *anon,
	}
	if *ns != "" {
		body["allowed_namespaces"] = splitComma(*ns)
	}
	if *ch != "" {
		body["allowed_channels"] = splitComma(*ch)
	}
	if *gitURL != "" {
		body["git"] = map[string]string{
			"url":    *gitURL,
			"branch": *gitBranch,
			"token":  *gitToken,
		}
	}

	data, err := client.Post("/api/repos", body)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	p.Success("리포지토리 '%s' 생성 완료", *name)
	return nil
}

// ─── repo get ─────────────────────────────────────────────────────────────────

func CmdRepoGet(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: repo get <name>")
	}
	name := args[0]
	data, err := client.Get("/api/repos/" + name)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var r struct {
		Name              string   `json:"name"`
		Owner             string   `json:"owner"`
		Description       string   `json:"description"`
		AnonymousAccess   string   `json:"anonymous_access"`
		Source            string   `json:"source"`
		MemberCount       int      `json:"member_count"`
		AllowedNamespaces []string `json:"allowed_namespaces"`
		AllowedChannels   []string `json:"allowed_channels"`
		GitURL            string   `json:"git_url"`
		GitBranch         string   `json:"git_branch"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}

	nsStr := "(제한 없음)"
	if len(r.AllowedNamespaces) > 0 {
		nsStr = strings.Join(r.AllowedNamespaces, ", ")
	}
	chStr := "(제한 없음)"
	if len(r.AllowedChannels) > 0 {
		chStr = strings.Join(r.AllowedChannels, ", ")
	}
	gitStr := "(비활성)"
	if r.GitURL != "" {
		gitStr = fmt.Sprintf("%s (branch: %s)", r.GitURL, r.GitBranch)
	}

	p.KV([][2]string{
		{"Name", r.Name},
		{"Owner", r.Owner},
		{"Description", r.Description},
		{"Anonymous Access", r.AnonymousAccess},
		{"Source", r.Source},
		{"Members", fmt.Sprintf("%d", r.MemberCount)},
		{"Allowed Namespaces", nsStr},
		{"Allowed Channels", chStr},
		{"Git Sync", gitStr},
	})
	return nil
}

// ─── repo update ──────────────────────────────────────────────────────────────

func CmdRepoUpdate(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: repo update <name> [옵션]")
	}
	name := args[0]

	fs := flag.NewFlagSet("repo update", flag.ContinueOnError)
	desc := fs.String("description", "", "설명 변경")
	anon := fs.String("anonymous", "", "익명 접근: none|read|write")
	ns := fs.String("namespaces", "", "허용 네임스페이스 (쉼표 구분, 비우면 제한 없음)")
	ch := fs.String("channels", "", "허용 채널 (쉼표 구분, 비우면 제한 없음)")
	gitURL := fs.String("git-url", "", "git 연동 URL (비우면 해제)")
	gitBranch := fs.String("git-branch", "main", "git 브랜치")
	gitToken := fs.String("git-token", "", "git 인증 토큰")
	clearGit := fs.Bool("clear-git", false, "git 연동 해제")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	body := map[string]any{}
	if *desc != "" {
		body["description"] = *desc
	}
	if *anon != "" {
		body["anonymous_access"] = *anon
	}
	if *ns != "" {
		body["allowed_namespaces"] = splitComma(*ns)
	}
	if *ch != "" {
		body["allowed_channels"] = splitComma(*ch)
	}
	if *clearGit || *gitURL == "" && fs.Lookup("git-url").Value.String() != "" {
		body["git"] = map[string]string{"url": ""}
	} else if *gitURL != "" {
		body["git"] = map[string]string{
			"url":    *gitURL,
			"branch": *gitBranch,
			"token":  *gitToken,
		}
	}

	if len(body) == 0 {
		return fmt.Errorf("변경할 항목이 없습니다. --description, --anonymous, --git-url 등을 지정하세요")
	}

	data, err := client.Patch("/api/repos/"+name, body)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	p.Success("리포지토리 '%s' 수정 완료", name)
	return nil
}

// ─── repo delete ──────────────────────────────────────────────────────────────

func CmdRepoDelete(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: repo delete <name> [--force]")
	}
	name := args[0]

	fs := flag.NewFlagSet("repo delete", flag.ContinueOnError)
	force := fs.Bool("force", false, "패키지가 있어도 삭제")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	path := "/api/repos/" + name
	if *force {
		path += "?force=true"
	}
	_, err := client.Delete(path)
	if err != nil {
		return err
	}
	p.Success("리포지토리 '%s' 삭제 완료", name)
	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
