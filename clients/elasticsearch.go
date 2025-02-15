package clients

import (
	"errors"
	es7 "github.com/elastic/go-elasticsearch/v7"
	es8 "github.com/elastic/go-elasticsearch/v8"
	"gopkg.in/olivere/elastic.v3"
)

type ElasticsearchClient interface {
	Ping() error
}

func NewElasticsearchClient(version int, url, username, password string) (ElasticsearchClient, error) {
	switch version {
	case 2:
		client, err := elastic.NewClient(elastic.SetURL(url), elastic.SetSniff(false), elastic.SetBasicAuth(username, password))
		if err != nil {
			return nil, err
		}
		return &Es2Client{Client: client, URL: url}, nil
	case 7:
		client, err := es7.NewClient(es7.Config{
			Addresses: []string{url},
			Username:  username,
			Password:  password,
		})
		if err != nil {
			return nil, err
		}
		return &Es7Client{Client: client}, nil
	case 8:
		client, err := es8.NewClient(es8.Config{
			Addresses: []string{url},
			Username:  username,
			Password:  password,
		})
		if err != nil {
			return nil, err
		}
		return &Es8Client{Client: client}, nil
	default:
		return nil, errors.New("unsupported Elasticsearch version")
	}
}
