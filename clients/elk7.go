package clients

import es7 "github.com/elastic/go-elasticsearch/v7"

type Es7Client struct {
	Client *es7.Client
}

func (e *Es7Client) Ping() error {
	res, err := e.Client.Ping()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
