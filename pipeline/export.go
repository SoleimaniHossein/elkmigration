package pipeline

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/utils"
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
)

// ExportDocuments exports documents from Elasticsearch 2.x, with state-saving to Redis.
// Accepts a mutex to prevent race conditions when accessing Redis.
func ExportDocuments(ctx context.Context, client clients.ElasticsearchClient, config *config.Config, docs chan<- *elastic.SearchResult, redis *clients.RedisClient) {
	defer close(docs)

	es2Client := client.(*clients.ES2Client).Client

	// Retrieve the last processed document ID from Redis
	offset, err := redis.Get(config.RedisKeyLastOffset)
	if err != nil {
		logger.Error("Error getting last offset", zap.Error(err))
		return
	}
	logger.Info(fmt.Sprintf("offset =>  %s", offset))
	// Initialize the scroll request
	scroll := es2Client.Scroll(config.ElkIndexFrom).Size(config.BulkSize).Scroll(config.ScrollTimeout)

	if offset != "" {
		logger.Info("Reached end of index")
		logger.Info(offset)
		scroll = scroll.ScrollId(offset)
	}

	for {
		// Execute the scroll request
		result, err := utils.RetryWithResult(ctx, config.MaxRetries, config.Timeout, func() (*elastic.SearchResult, error) {
			return scroll.Do()
		})

		if err != nil {
			logger.Error("Error during scroll", zap.Error(err))
			return
		}

		// Check if we've reached the end of the scroll
		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			return
		}

		// Send the result to the output channel
		docs <- result

		// Save the latest ScrollID to Redis
		if err := redis.Set(config.RedisKeyLastOffset, result.ScrollId, config.Timeout); err != nil {
			logger.Error("Failed to save last ScrollID to Redis", zap.Error(err))
		} else {
			logger.Info("Updated ScrollID in Redis", zap.String("ScrollID", result.ScrollId))
		}
		// Update the scroll request with the latest ScrollID
		scroll = es2Client.Scroll(config.ElkIndexFrom).Size(config.BulkSize).ScrollId(result.ScrollId).Scroll(config.ScrollTimeout)

	}
}
