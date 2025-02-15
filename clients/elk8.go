package clients

import (
	es8 "github.com/elastic/go-elasticsearch/v8"
)

var _ ElasticsearchClient = (*Es8Client)(nil)

type Es8Client struct {
	Client *es8.Client
}

func (e *Es8Client) Ping() error {
	res, err := e.Client.Ping()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
