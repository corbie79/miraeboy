package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"
)

// CmdAuditList handles "mboy audit list [flags]"
func CmdAuditList(client *Client, p *Printer, args []string) error {
	fs := flag.NewFlagSet("audit list", flag.ContinueOnError)
	repo   := fs.String("repo", "", "리포지토리 필터")
	user   := fs.String("user", "", "사용자 필터")
	action := fs.String("action", "", "액션 필터 (upload, download, delete, login, ...)")
	since  := fs.String("since", "", "시작 시간 (RFC3339, 예: 2026-01-01T00:00:00Z)")
	until  := fs.String("until", "", "종료 시간 (RFC3339)")
	limit  := fs.Int("limit", 50, "최대 항목 수 (최대 1000)")
	page   := fs.Int("page", 1, "페이지 번호")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/audit?limit=%d&page=%d", *limit, *page)
	if *repo != "" {
		path += "&repo=" + *repo
	}
	if *user != "" {
		path += "&user=" + *user
	}
	if *action != "" {
		path += "&action=" + *action
	}
	if *since != "" {
		path += "&since=" + *since
	}
	if *until != "" {
		path += "&until=" + *until
	}

	data, err := client.Get(path)
	if err != nil {
		return err
	}

	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Entries []struct {
			Timestamp time.Time `json:"ts"`
			Username  string    `json:"user"`
			Action    string    `json:"action"`
			Repo      string    `json:"repo"`
			Package   string    `json:"pkg"`
			IP        string    `json:"ip"`
			Detail    string    `json:"detail"`
		} `json:"entries"`
		Total int `json:"total"`
		Page  int `json:"page"`
		Pages int `json:"pages"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	fmt.Printf("감사 로그 (총 %d개, 페이지 %d/%d)\n\n", resp.Total, resp.Page, resp.Pages)

	headers := []string{"시간", "사용자", "액션", "리포지토리", "패키지", "IP"}
	rows := make([][]string, len(resp.Entries))
	for i, e := range resp.Entries {
		rows[i] = []string{
			e.Timestamp.Local().Format("01-02 15:04:05"),
			e.Username,
			e.Action,
			e.Repo,
			e.Package,
			e.IP,
		}
	}
	p.Table(headers, rows)
	return nil
}
