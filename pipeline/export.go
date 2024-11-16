package pipeline

import (
	"context"
	"elkmigration/clients"
	"elkmigration/logger"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
)

const (
	redisKeyLastID = "elasticsearch:last_processed_id"
	bulkSize       = 500
	maxRetries     = 6
	initialDelay   = 1 * time.Second
	scrollTimeout  = "20s" // Set a scroll timeout to maintain context on the server
)

// ExportDocuments exports documents from Elasticsearch 2.x, with state-saving to Redis.
// Accepts a mutex to prevent race conditions when accessing Redis.
func ExportDocuments(client clients.ElasticsearchClient, index string, docs chan<- map[string]interface{}, redisClient *redis.Client, mu *sync.Mutex) {
	ctx := context.Background()
	es2Client := client.(*clients.ES2Client).Client

	// Retrieve last processed document ID from Redis
	mu.Lock()
	lastID, err := getLastProcessedID(ctx, redisClient)
	mu.Unlock()
	if err != nil {
		logger.Error("Failed to retrieve last processed ID from Redis", zap.Error(err))
		return
	}
	resume := lastID != ""

	// Initialize Elasticsearch scroll with timeout and size
	scroll := es2Client.Scroll(index).Size(bulkSize).Scroll(scrollTimeout)

	for {
		// Execute scroll with retries and exponential backoff
		var result *elastic.SearchResult
		retries := 0
		for {
			result, err = scroll.Do()
			if err == nil {
				break
			}
			if retries >= maxRetries {
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
			if err := saveLastProcessedID(ctx, redisClient, hit.Id); err != nil {
				logger.Error("Failed to save last processed ID to Redis", zap.Error(err))
			}
			mu.Unlock()

			logger.Warn("Exported document", zap.Int("idx", idx), zap.String("hit ID", hit.Id))
		}

		// Update the scroll with the current scroll ID
		scroll = es2Client.Scroll(index).Size(bulkSize).ScrollId(result.ScrollId).Scroll(scrollTimeout)
	}
	close(docs)
}

// getLastProcessedID retrieves the last processed document ID from Redis.
func getLastProcessedID(ctx context.Context, redisClient *redis.Client) (string, error) {
	lastID, err := redisClient.Get(ctx, redisKeyLastID).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // Start from scratch if no ID is stored
	} else if err != nil {
		return "", err
	}
	return lastID, nil
}

// saveLastProcessedID saves the last processed document ID to Redis.
func saveLastProcessedID(ctx context.Context, redisClient *redis.Client, docID string) error {
	return redisClient.Set(ctx, redisKeyLastID, docID, 0).Err()
}
