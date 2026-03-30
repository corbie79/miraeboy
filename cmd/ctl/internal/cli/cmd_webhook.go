package cli

import (
	"encoding/json"
	"flag"
	"fmt"
)

// CmdWebhook handles "mboy webhook <sub> ..."
func CmdWebhook(client *Client, p *Printer, sub string, args []string) error {
	switch sub {
	case "list":
		return webhookList(client, p, args)
	case "create":
		return webhookCreate(client, p, args)
	case "delete":
		return webhookDelete(client, p, args)
	case "test":
		return webhookTest(client, p, args)
	default:
		return fmt.Errorf("webhook 서브커맨드: list | create | delete | test\n사용법: mboy webhook <sub> <repo> [옵션]")
	}
}

func webhookList(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: mboy webhook list <repo>")
	}
	data, err := client.Get("/api/repos/" + args[0] + "/webhooks")
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	var resp struct {
		Webhooks []struct {
			ID     string   `json:"id"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Active bool     `json:"active"`
		} `json:"webhooks"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Webhooks) == 0 {
		fmt.Println("(웹훅 없음)")
		return nil
	}
	rows := make([][]string, len(resp.Webhooks))
	for i, wh := range resp.Webhooks {
		active := "active"
		if !wh.Active {
			active = "disabled"
		}
		rows[i] = []string{wh.ID, wh.URL, fmt.Sprintf("%v", wh.Events), active}
	}
	p.Table([]string{"ID", "URL", "EVENTS", "STATUS"}, rows)
	return nil
}

func webhookCreate(client *Client, p *Printer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("사용법: mboy webhook create <repo> --url URL [--events e1,e2] [--secret S]")
	}
	repo := args[0]
	fs := flag.NewFlagSet("webhook create", flag.ContinueOnError)
	url    := fs.String("url", "", "웹훅 URL (필수)")
	events := fs.String("events", "*", "이벤트 (콤마 구분: package.upload,cargo.publish,...)")
	secret := fs.String("secret", "", "HMAC-SHA256 서명 시크릿")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *url == "" {
		return fmt.Errorf("--url 은 필수입니다")
	}
	body := map[string]any{
		"url":    *url,
		"events": splitComma(*events),
	}
	if *secret != "" {
		body["secret"] = *secret
	}
	data, err := client.Post("/api/repos/"+repo+"/webhooks", body)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	var wh struct{ ID string `json:"id"` }
	_ = json.Unmarshal(data, &wh)
	p.Success("웹훅 생성 완료 (id: %s)", wh.ID)
	return nil
}

func webhookDelete(client *Client, p *Printer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("사용법: mboy webhook delete <repo> <id>")
	}
	_, err := client.Delete("/api/repos/" + args[0] + "/webhooks/" + args[1])
	if err != nil {
		return err
	}
	p.Success("웹훅 삭제 완료")
	return nil
}

func webhookTest(client *Client, p *Printer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("사용법: mboy webhook test <repo> <id>")
	}
	data, err := client.Post("/api/repos/"+args[0]+"/webhooks/"+args[1]+"/test", nil)
	if err != nil {
		return err
	}
	if p.jsonMode {
		p.Raw(data)
		return nil
	}
	var resp struct{ DeliveryID string `json:"delivery_id"` }
	_ = json.Unmarshal(data, &resp)
	p.Success("ping 전송됨 (delivery_id: %s)", resp.DeliveryID)
	return nil
}
