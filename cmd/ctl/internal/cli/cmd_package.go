package cli

import (
	"encoding/json"
	"fmt"
)

func CmdPackageSearch(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: package search <repo> [query]")
	}
	repo := args[0]
	query := "*"
	if len(args) > 1 {
		query = args[1]
	}

	data, err := client.Get(fmt.Sprintf("/api/conan/%s/v2/conans/search?q=%s", repo, query))
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Results []string `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if len(resp.Results) == 0 {
		fmt.Println("(패키지 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Results))
	for i, ref := range resp.Results {
		rows[i] = []string{ref}
	}
	p.Table([]string{"PACKAGE"}, rows)
	return nil
}
