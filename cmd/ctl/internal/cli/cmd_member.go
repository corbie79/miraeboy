package cli

import (
	"encoding/json"
	"fmt"
)

func CmdMemberList(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: member list <repo>")
	}
	repo := args[0]
	data, err := client.Get("/api/repos/" + repo + "/members")
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Members []struct {
			Username   string `json:"username"`
			Permission string `json:"permission"`
			IsOwner    bool   `json:"is_owner"`
		} `json:"members"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if len(resp.Members) == 0 {
		fmt.Println("(멤버 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Members))
	for i, m := range resp.Members {
		owner := ""
		if m.IsOwner {
			owner = "owner"
		}
		rows[i] = []string{m.Username, m.Permission, owner}
	}
	p.Table([]string{"USERNAME", "PERMISSION", "ROLE"}, rows)
	return nil
}

func CmdMemberAdd(client *Client, p *Printer, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("사용법: member add <repo> <username> <permission>")
	}
	repo, user, perm := args[0], args[1], args[2]
	_, err := client.Post("/api/repos/"+repo+"/members", map[string]string{
		"username":   user,
		"permission": perm,
	})
	if err != nil {
		return err
	}
	p.Success("'%s'을(를) %s 에 %s 권한으로 추가", user, repo, perm)
	return nil
}

func CmdMemberUpdate(client *Client, p *Printer, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("사용법: member update <repo> <username> <permission>")
	}
	repo, user, perm := args[0], args[1], args[2]
	_, err := client.Put("/api/repos/"+repo+"/members/"+user, map[string]string{
		"permission": perm,
	})
	if err != nil {
		return err
	}
	p.Success("'%s'의 %s 권한을 %s 로 변경", user, repo, perm)
	return nil
}

func CmdMemberRemove(client *Client, p *Printer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("사용법: member remove <repo> <username>")
	}
	repo, user := args[0], args[1]
	_, err := client.Delete("/api/repos/" + repo + "/members/" + user)
	if err != nil {
		return err
	}
	p.Success("'%s'을(를) %s 에서 제거", user, repo)
	return nil
}
