package cli

import (
	"encoding/json"
	"fmt"
)

func CmdCargoSearch(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: cargo search <repo> [query]")
	}
	repo := args[0]
	query := ""
	if len(args) > 1 {
		query = args[1]
	}

	data, err := client.Get(fmt.Sprintf("/cargo/%s/api/v1/crates?q=%s", repo, query))
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Crates []struct {
			Name       string `json:"name"`
			MaxVersion string `json:"max_version"`
			Description string `json:"description"`
		} `json:"crates"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Crates) == 0 {
		fmt.Println("(크레이트 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Crates))
	for i, c := range resp.Crates {
		rows[i] = []string{c.Name, c.MaxVersion, c.Description}
	}
	p.Table([]string{"NAME", "VERSION", "DESCRIPTION"}, rows)
	return nil
}

func CmdCargoYank(client *Client, p *Printer, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("사용법: cargo yank <repo> <name> <version>")
	}
	repo, name, version := args[0], args[1], args[2]
	_, err := client.Delete(fmt.Sprintf("/cargo/%s/api/v1/crates/%s/%s/yank", repo, name, version))
	if err != nil {
		return err
	}
	p.Success("크레이트 %s@%s yanked (repo: %s)", name, version, repo)
	return nil
}

func CmdCargoUnyank(client *Client, p *Printer, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("사용법: cargo unyank <repo> <name> <version>")
	}
	repo, name, version := args[0], args[1], args[2]
	_, err := client.Put(fmt.Sprintf("/cargo/%s/api/v1/crates/%s/%s/unyank", repo, name, version), nil)
	if err != nil {
		return err
	}
	p.Success("크레이트 %s@%s unyanked (repo: %s)", name, version, repo)
	return nil
}
