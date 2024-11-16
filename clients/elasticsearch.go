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

type ES2Client struct {
	Client *elastic.Client
	URL    string
}

func (e *ES2Client) Ping() error {
	_, _, err := e.Client.Ping(e.URL).Do()
	return err
}

type ES7Client struct {
	Client *es7.Client
}

func (e *ES7Client) Ping() error {
	res, err := e.Client.Ping()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

type ES8Client struct {
	Client *es8.Client
}

func (e *ES8Client) Ping() error {
	res, err := e.Client.Ping()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func NewElasticsearchClient(version int, url string) (ElasticsearchClient, error) {
	switch version {
	case 2:
		client, err := elastic.NewClient(elastic.SetURL(url), elastic.SetSniff(false))
		if err != nil {
			return nil, err
		}
		return &ES2Client{Client: client, URL: url}, nil
	case 7:
		client, err := es7.NewClient(es7.Config{
			Addresses: []string{url},
		})
		if err != nil {
			return nil, err
		}
		return &ES7Client{Client: client}, nil
	case 8:
		client, err := es8.NewClient(es8.Config{
			Addresses: []string{url},
		})
		if err != nil {
			return nil, err
		}
		return &ES8Client{Client: client}, nil
	default:
		return nil, errors.New("unsupported Elasticsearch version")
	}
}
