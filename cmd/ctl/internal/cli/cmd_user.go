package cli

import (
	"encoding/json"
	"flag"
	"fmt"
)

func CmdUserList(client *Client, p *Printer, args []string) error {
	data, err := client.Get("/api/users")
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}

	var resp struct {
		Users []struct {
			Username  string `json:"username"`
			Admin     bool   `json:"admin"`
			Source    string `json:"source"`
			CreatedAt string `json:"created_at"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Users) == 0 {
		fmt.Println("(유저 없음)")
		return nil
	}

	rows := make([][]string, len(resp.Users))
	for i, u := range resp.Users {
		admin := ""
		if u.Admin {
			admin = "admin"
		}
		rows[i] = []string{u.Username, admin, u.Source, u.CreatedAt[:10]}
	}
	p.Table([]string{"USERNAME", "ROLE", "SOURCE", "CREATED"}, rows)
	return nil
}

func CmdUserCreate(client *Client, p *Printer, args []string) error {
	fs := flag.NewFlagSet("user create", flag.ContinueOnError)
	username := fs.String("username", "", "사용자 이름 (필수)")
	password := fs.String("password", "", "비밀번호 (필수, 6자 이상)")
	admin := fs.Bool("admin", false, "관리자 권한 부여")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return fmt.Errorf("--username 이 필요합니다")
	}
	if *password == "" {
		return fmt.Errorf("--password 가 필요합니다")
	}

	data, err := client.Post("/api/users", map[string]any{
		"username": *username,
		"password": *password,
		"admin":    *admin,
	})
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	role := "user"
	if *admin {
		role = "admin"
	}
	p.Success("유저 '%s' 생성 완료 (role: %s)", *username, role)
	return nil
}

func CmdUserUpdate(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: user update <username> [옵션]")
	}
	username := args[0]

	fs := flag.NewFlagSet("user update", flag.ContinueOnError)
	password := fs.String("password", "", "새 비밀번호")
	admin := fs.String("admin", "", "관리자 권한: true | false")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	body := map[string]any{}
	if *password != "" {
		body["password"] = *password
	}
	if *admin == "true" {
		body["admin"] = true
	} else if *admin == "false" {
		body["admin"] = false
	}
	if len(body) == 0 {
		return fmt.Errorf("변경할 항목이 없습니다. --password 또는 --admin true|false 를 지정하세요")
	}

	data, err := client.Patch("/api/users/"+username, body)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	p.Success("유저 '%s' 수정 완료", username)
	return nil
}

func CmdUserDelete(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: user delete <username>")
	}
	username := args[0]
	if _, err := client.Delete("/api/users/" + username); err != nil {
		return err
	}
	p.Success("유저 '%s' 삭제 완료", username)
	return nil
}
