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

// ExportDocuments exports documents from Elasticsearch 2.x using a range query and scroll API.
func ExportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, docs chan<- *elastic.SearchResult, redis *clients.RedisClient) {
	defer close(docs)

	es2Client := client.(*clients.Es2Client).Client

	// Fetch Scroll ID from Redis (for pagination)
	scrollID, err := redis.Get(config.Redis.KeyScrollID)
	if err != nil {
		logger.Warn("Failed to get Scroll ID from Redis", zap.Error(err))
	}

	// Construct Range Query (fetch documents within a date range)
	rangeQuery := elastic.NewRangeQuery(config.App.SortBy).
		From(config.App.StartDate).
		To(config.App.EndDate)

	// Log query for debugging
	queryJSON, _ := rangeQuery.Source()
	logger.Info("Generated Range Query", zap.Any("query", queryJSON))

	var scrollService *elastic.ScrollService
	if scrollID == "" {
		logger.Info("Starting from the beginning...")
		scrollService = es2Client.Scroll(config.Elk2.Index).
			Size(config.App.BulkSize).
			Query(rangeQuery). // Use range query instead of MatchAllQuery
			Sort(config.App.SortBy, config.App.Asc).
			Scroll(config.App.ScrollTTL) // Keep scroll alive for 2 minutes
	} else {
		logger.Info("Resuming from Scroll ID", zap.String("scrollID", scrollID))
		scrollService = es2Client.Scroll(config.Elk2.Index).
			ScrollId(scrollID).
			Scroll(config.App.ScrollTTL) // Keep scroll alive
	}

	for {
		// Fetch next batch with retries
		result, err := utils.RetryWithResult(ctx, config.App.MaxRetries, config.App.TTL, func() (*elastic.SearchResult, error) {
			return scrollService.Do()
		})

		if errors.Is(err, elastic.EOS) {
			logger.Info("No more documents to export.", zap.Error(err))
			return
		}

		if err != nil {
			logger.Error("Scroll API error", zap.Error(err))
			return
		}

		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			return
		}

		// Send batch to channel
		docs <- result

		// Save new Scroll ID to Redis
		err = redis.Set(config.Redis.KeyScrollID, result.ScrollId, config.Redis.TTL)
		if err != nil {
			logger.Warn("Failed to save Scroll ID to Redis", zap.Error(err))
			return
		}

		logger.Info("Processed batch, Scroll ID updated.", zap.String("scrollID", result.ScrollId))
	}
}
