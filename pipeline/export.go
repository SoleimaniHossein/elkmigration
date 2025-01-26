package pipeline

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/utils"
	"errors"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
	"log"
	"sync"
)

// ExportDocuments exports documents from Elasticsearch 2.x, with state-saving to Redis.
// Accepts a mutex to prevent race conditions when accessing Redis.
func ExportDocuments(ctx context.Context, client clients.ElasticsearchClient, config *config.Config, docs chan<- *elastic.SearchResult, redis *clients.RedisClient, mu *sync.Mutex) {
	defer close(docs)

	es2Client := client.(*clients.ES2Client).Client

	scrollID, err := redis.Get(config.RedisKeyScrollID)
	if err != nil {
		logger.Warn("failed to get offset from Redis: %w", zap.Error(err))
	}

	logger.Info("Starting from scrollID: %d\n", zap.String("scrollID", scrollID))

	var scrollService *elastic.ScrollService
	if scrollID == "" {
		// Start a new scroll if no previous scroll ID exists
		scrollService = es2Client.Scroll(config.ElkIndexFrom).
			Size(config.BulkSize).
			Query(elastic.NewMatchAllQuery()) // Match all documents
	} else {
		// Resume an existing scroll
		scrollService = es2Client.Scroll(config.ElkIndexFrom).
			ScrollId(scrollID)
	}

	for {

		// Execute the scroll request
		result, err := utils.RetryWithResult(ctx, config.MaxRetries, config.TTL, func() (*elastic.SearchResult, error) {
			return scrollService.Do()
		})

		if errors.Is(err, elastic.EOS) {
			log.Println("No more documents to export.")
			break
		}

		if err != nil {
			logger.Error("scroll API error: %w", zap.Error(err))
			return
		}

		// Check if we've reached the end of the scroll
		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			break
		}

		// Send the result to the output channel
		docs <- result

		// Save the updated offset to Redis
		mu.Lock()
		err = redis.Set(config.RedisKeyScrollID, result.ScrollId, config.RedisTTL)
		if err != nil {
			logger.Warn("failed to save offset to Redis", zap.Error(err))
			return
		}
		mu.Unlock()

		logger.Info("Processed batch. Scroll ID saved.", zap.String("scrollID", result.ScrollId))

	}
}
