package clients

import (
	"elkmigration/logger"
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
)

type Es2Client struct {
	Client *elastic.Client
	URLs   []string
}

// Ping checks the health of Elasticsearch, ignoring failed URLs if at least one is reachable
func (e *Es2Client) Ping() error {
	successfulPings := 0

	for _, url := range e.URLs {
		_, _, err := e.Client.Ping(url).Do()
		if err != nil {
			logger.Info("Failed to ping ", zap.String("url:", url), zap.Error(err))
			continue // Ignore the failed ping and move to the next URL
		}

		fmt.Printf("Successfully pinged %s\n", url)
		successfulPings++
	}

	if successfulPings == 0 {
		return fmt.Errorf("none of the Elasticsearch URLs are reachable")
	}

	return nil // At least one URL responded successfully
}
