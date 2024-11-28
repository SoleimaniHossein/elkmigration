package pipeline

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/utils"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
)

const (
	initialDelay = 1 * time.Second
)

// ExportDocuments exports documents from Elasticsearch 2.x, with state-saving to Redis.
// Accepts a mutex to prevent race conditions when accessing Redis.
func ExportDocuments(client clients.ElasticsearchClient, config *config.Config, docs chan<- map[string]interface{}, redis *clients.Redis, mu *sync.Mutex) {
	var ctx = context.Background()

	es2Client := client.(*clients.ES2Client).Client

	// Retrieve last processed document ID from Redis
	mu.Lock()
	lastID, err := redis.Get(ctx, config.RedisKeyLastID)
	//lastCount, _ := redis.Get(ctx, config.RedisKeyCount)
	lastOffset, _ := redis.Get(ctx, config.RedisKeyLastOffset)
	mu.Unlock()

	if err != nil {
		logger.Info("Start Process from Beginning", zap.Error(err))
	}

	resume := lastID != nil || lastOffset != nil

	// Initialize Elasticsearch scroll with timeout and size
	scroll := es2Client.Scroll(config.ElkIndexFrom).Size(config.BulkSize).Scroll(config.ScrollTimeout)

	for {
		// Execute scroll with retries and exponential backoff
		var result *elastic.SearchResult
		retries := 0
		for {
			result, err = scroll.Do()
			if err == nil {
				break
			}
			if retries >= config.MaxRetries {
				logger.Error("Max retries reached during scroll execution", zap.Error(err))
				return
			}
			logger.Warn("Scroll execution error, retrying", zap.Int("attempt", retries+1), zap.Error(err))
			time.Sleep(time.Duration(1<<retries) * initialDelay) // Exponential backoff
			retries++
		}

		// Check if the scroll has reached the end
		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			break
		}

		for idx, hit := range result.Hits.Hits {
			// Skip documents until we reach the one after lastID on recovery
			if resume && hit.Id == lastID {
				resume = false
				continue
			} else if resume {
				continue
			}

			// Process the document
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				logger.Warn("Error unmarshalling document", zap.Error(err))
				continue
			}
			docs <- doc // Send document to the next stage

			// Save the last processed document ID to Redis with a mutex lock
			mu.Lock()
			if err := redis.Save(ctx, config.RedisKeyLastOffset, result.ScrollId); err != nil {
				logger.Error("Failed to save last Offset to Redis", zap.Error(err))
			}
			//if err := redis.Save(ctx, config.RedisKeyCount, strconv.Itoa(utils.StringToIntOrDefault(lastCount, 0)+1)); err != nil {
			if err := redis.Save(ctx, config.RedisKeyCount, utils.IntToString(int(result.Hits.TotalHits))); err != nil {
				logger.Error("Failed to save last count of Docs to Redis", zap.Error(err))
			}

			if err := redis.Save(ctx, config.RedisKeyLastID, hit.Id); err != nil {
				logger.Error("Failed to save last ID to Redis", zap.Error(err))
			}

			if err := redis.SaveJSON(ctx, config.RedisKeyLastDoc, doc); err != nil {
				logger.Error("Failed to save last Doc to Redis", zap.Error(err))
			}
			mu.Unlock()

			logger.Info("Exported document", zap.Int("idx", idx), zap.String("hit ID", hit.Id), zap.String("last Offset", result.ScrollId), zap.String("last Docs Count", "test"), zap.String("last Doc ", "in your redis..."))
		}

		// Update the scroll with the current scroll ID
		scroll = es2Client.Scroll(config.ElkIndexFrom).Size(config.BulkSize).ScrollId(result.ScrollId).Scroll(config.ScrollTimeout)
	}
	close(docs)
}
