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
)

// ExportDocuments exports documents from Elasticsearch 2.x, with state-saving to Redis.
// Accepts a mutex to prevent race conditions when accessing Redis.
func ExportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, docs chan<- *elastic.SearchResult, redis *clients.RedisClient) {
	defer close(docs)

	es2Client := client.(*clients.Es2Client).Client

	scrollID, err := redis.Get(config.Redis.KeyScrollID)

	if err != nil {
		logger.Warn("failed to get offset from Redis: %w", zap.Error(err))
	}

	var scrollService *elastic.ScrollService
	if scrollID == "" {
		logger.Info("Starting from the beginning...")
		scrollService = es2Client.Scroll(config.Elk2.Index).
			Size(config.App.BulkSize).
			Query(elastic.NewMatchAllQuery()).
			Sort(config.Elk2.SortBy, config.Elk2.Asc)
	} else {
		logger.Info("Starting from: ", zap.String("scrollID", scrollID))
		scrollService = es2Client.Scroll(config.Elk2.Index).
			ScrollId(scrollID)
		//ScrollId(scrollID).Sort(config.Elk2.SortBy,config.Elk2.Asc)
	}

	for {
		result, err := utils.RetryWithResult(ctx, config.App.MaxRetries, config.App.TTL, func() (*elastic.SearchResult, error) {
			return scrollService.Do()
		})

		if errors.Is(err, elastic.EOS) {
			logger.Info("No more documents to export.", zap.Error(err))
			return
		}

		if err != nil {
			logger.Error("scroll API error: %w", zap.Error(err))
			return
		}

		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			return
		}

		docs <- result

		err = redis.Set(config.Redis.KeyScrollID, result.ScrollId, config.Redis.TTL)

		if err != nil {
			logger.Warn("failed to save offset to Redis", zap.Error(err))
			return
		}

		//logger.Info("Processed batch. Scroll ID saved.", zap.String("scrollID", result.ScrollId))

	}
}
