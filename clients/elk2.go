package clients

import "gopkg.in/olivere/elastic.v3"

type Es2Client struct {
	Client *elastic.Client
	URL    string
}

func (e *Es2Client) Ping() error {
	_, _, err := e.Client.Ping(e.URL).Do()
	return err
}
