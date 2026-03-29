package cli

import (
	"encoding/json"
	"flag"
	"fmt"
)

func CmdBuildList(client *Client, p *Printer, args []string) error {
	data, err := client.Get("/api/builds")
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Builds []struct {
			ID        string `json:"id"`
			Repo      string `json:"repo"`
			Status    string `json:"status"`
			Platform  string `json:"platform"`
			CreatedAt string `json:"created_at"`
		} `json:"builds"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if len(resp.Builds) == 0 {
		fmt.Println("(빌드 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Builds))
	for i, b := range resp.Builds {
		rows[i] = []string{b.ID, b.Repo, b.Platform, b.Status, b.CreatedAt}
	}
	p.Table([]string{"ID", "REPO", "PLATFORM", "STATUS", "CREATED"}, rows)
	return nil
}

func CmdBuildTrigger(client *Client, p *Printer, args []string) error {
	fs := flag.NewFlagSet("build trigger", flag.ContinueOnError)
	repo := fs.String("repo", "", "리포지토리 이름 (필수)")
	gitURL := fs.String("git-url", "", "소스 git URL (필수)")
	gitRef := fs.String("ref", "main", "git 브랜치 또는 태그")
	platforms := fs.String("platforms", "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64", "빌드 플랫폼 (쉼표 구분)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" {
		return fmt.Errorf("--repo 가 필요합니다")
	}
	if *gitURL == "" {
		return fmt.Errorf("--git-url 이 필요합니다")
	}

	body := map[string]any{
		"repo":      *repo,
		"git_url":   *gitURL,
		"ref":       *gitRef,
		"platforms": splitComma(*platforms),
	}

	data, err := client.Post("/api/builds", body)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var result struct{ ID string `json:"id"` }
	if err := json.Unmarshal(data, &result); err != nil {
		p.Raw(data)
		return nil
	}
	p.Success("빌드 트리거 완료 (ID: %s)", result.ID)
	fmt.Printf("  빌드 상태: mboy build get %s\n", result.ID)
	return nil
}

func CmdBuildGet(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: build get <id>")
	}
	data, err := client.Get("/api/builds/" + args[0])
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var b struct {
		ID        string   `json:"id"`
		Repo      string   `json:"repo"`
		GitURL    string   `json:"git_url"`
		Ref       string   `json:"ref"`
		Status    string   `json:"status"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		Artifacts []string `json:"artifacts"`
		Error     string   `json:"error"`
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}

	artifacts := "(없음)"
	if len(b.Artifacts) > 0 {
		artifacts = fmt.Sprintf("%d개", len(b.Artifacts))
		for _, a := range b.Artifacts {
			artifacts += "\n  - " + a
		}
	}
	errStr := ""
	if b.Error != "" {
		errStr = b.Error
	}

	pairs := [][2]string{
		{"ID", b.ID},
		{"Repo", b.Repo},
		{"Git URL", b.GitURL},
		{"Ref", b.Ref},
		{"Status", b.Status},
		{"Created", b.CreatedAt},
		{"Updated", b.UpdatedAt},
		{"Artifacts", artifacts},
	}
	if errStr != "" {
		pairs = append(pairs, [2]string{"Error", errStr})
	}
	p.KV(pairs)
	return nil
}
