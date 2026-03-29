package cli

import "encoding/json"

func CmdStatus(client *Client, p *Printer) error {
	data, err := client.Get("/ping")
	if err != nil {
		return err
	}
	var v any
	if json.Unmarshal(data, &v) != nil {
		v = string(data)
	}
	p.JSON(v)
	return nil
}
