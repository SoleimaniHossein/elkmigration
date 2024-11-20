package pipeline

import (
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
	es2Client := client.(*clients.ES2Client).Client

	// Retrieve last processed document ID from Redis
	mu.Lock()
	lastID, err := redis.GetLastProcessedID(config.RedisKeyLastID)
	lastDoc, err := redis.GetLastProcessedID(config.RedisKeyLastDoc)
	lastCount, err := redis.GetLastProcessedID(config.RedisKeyCount)
	mu.Unlock()

	if err != nil {
		logger.Error("Failed to retrieve last processed ID from Redis", zap.Error(err))
		return
	}

	resume := lastID != ""

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
			if err := redis.SaveLastProcessedID(config.RedisKeyLastID, hit.Id); err != nil {
				logger.Error("Failed to save last processed ID to Redis", zap.Error(err))
			}
			if err := redis.SaveLastProcessedID(config.RedisKeyCount, utils.StringToIntOrDefault(config.RedisKeyCount, 0)+1); err != nil {
				logger.Error("Failed to save last Docs count to Redis", zap.Error(err))
			}
			mu.Unlock()

			logger.Info("Exported document", zap.Int("idx", idx), zap.String("hit ID", hit.Id), zap.String("last Docs Count", lastCount), zap.String("last Doc", lastDoc))
		}

		// Update the scroll with the current scroll ID
		scroll = es2Client.Scroll(config.ElkIndexFrom).Size(config.BulkSize).ScrollId(result.ScrollId).Scroll(config.ScrollTimeout)
	}
	close(docs)
}
