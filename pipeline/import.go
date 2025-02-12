//package pipeline
//
//import (
//	"bytes"
//	"context"
//	"elkmigration/clients"
//	"elkmigration/config"
//	"elkmigration/logger"
//	"elkmigration/utils"
//	"encoding/json"
//	"errors"
//	"fmt"
//	es8 "github.com/elastic/go-elasticsearch/v8"
//	"go.uber.org/zap"
//	"strconv"
//	"sync/atomic"
//)
//
//var totalProcessed int64 = 0
//
//func ImportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, redisClient *clients.RedisClient, transformedDocs <-chan map[string]interface{}) {
//	redisClient.Ctx = ctx
//
//	esClient, ok := client.(*clients.ES8Client) // Type assertion for ES8Client
//
//	if !ok {
//		logger.Error("Invalid client type; expected *ES8Client")
//		return
//	}
//
//	_, err := esClient.Client.Indices.Exists([]string{config.Elk8.Index}, esClient.Client.Indices.Exists.WithContext(ctx))
//	if err != nil {
//		logger.Error("Error checking if index exists", zap.Error(err))
//		return
//	}
//
//	bulkData := make([]map[string]interface{}, 0, config.App.BulkSize)
//
//	for doc := range transformedDocs {
//		bulkData = append(bulkData, doc)
//
//		// Send bulk request when reaching the bulkSize
//		if len(bulkData) >= config.App.BulkSize {
//
//			err = utils.Retry(ctx, config.App.MaxRetries, config.App.TTL, func() error {
//				return sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, bulkData, config.App.MaxBulkPayloadBytes)
//			})
//			if err != nil {
//				logger.Error("Error sending bulk request", zap.Error(err))
//			}
//			bulkData = bulkData[:0] // Reset the bulk data buffer
//		}
//	}
//
//	// Send any remaining documents
//	if len(bulkData) > 0 {
//		if err := sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, bulkData, config.App.MaxBulkPayloadBytes); err != nil {
//			logger.Error("Error during final bulk insert", zap.Error(err))
//		}
//	}
//}
//
//func sendBulkRequest(config *config.Config, client *es8.Client, redisClient *clients.RedisClient, index string, bulkData []map[string]interface{}, maxBulkPayloadBytes int) error {
//	var buf bytes.Buffer
//	encoder := json.NewEncoder(&buf)
//	totalProcessed = updateTotalProcessed(redisClient, config.Redis)
//	// Prepare bulk request format
//	for _, doc := range bulkData {
//
//		atomic.AddInt64(&totalProcessed, 1)
//
//		err := redisClient.Set(config.Redis.KeyTotalProcessed, totalProcessed, config.Redis.TTL)
//
//		if err != nil {
//			logger.Warn("failed to save totalProcessed to Redis", zap.Error(err))
//			return err
//		}
//
//		meta := map[string]interface{}{
//			"index": map[string]interface{}{
//				"_index": index,
//			},
//		}
//		if err := encoder.Encode(meta); err != nil {
//			return err
//		}
//		if err := encoder.Encode(doc); err != nil {
//			return err
//		}
//
//		// Check if the payload size exceeds the limit
//		if buf.Len() >= maxBulkPayloadBytes {
//			if err := executeBulkRequest(client, buf.Bytes()); err != nil {
//				return err
//			}
//			buf.Reset() // Reset buffer for the next batch
//		}
//	}
//
//	// Send remaining documents
//	if buf.Len() > 0 {
//		if err := executeBulkRequest(client, buf.Bytes()); err != nil {
//			logger.Warn("executeBulkRequest err: %s", zap.Error(err))
//		}
//	}
//
//	logger.Info("Bulk request completed", zap.Int64("documents_count", totalProcessed))
//
//	return nil
//}
//
//func executeBulkRequest(client *es8.Client, bulkPayload []byte) error {
//
//	res, err := client.Bulk(bytes.NewReader(bulkPayload))
//	if err != nil {
//		logger.Error("Failed to execute bulk request", zap.Error(err))
//		return err
//	}
//	defer res.Body.Close()
//	// Check for errors in the response
//	if res.IsError() {
//		logger.Error("Bulk request failed when importing", zap.String("status", res.Status()))
//		return errors.New("bulk request failed")
//	}
//	logger.Info("execute BulkRequest completed", zap.Any("client", client.Info))
//
//	return nil
//}
//
//func updateTotalProcessed(redisClient *clients.RedisClient, redisConfig config.Redis) int64 {
//	stringValue, err := redisClient.Get(redisConfig.KeyTotalProcessed)
//
//	if err != nil {
//		logger.Warn("not found total processed", zap.Error(err))
//		return 0
//	}
//
//	val, err := strconv.ParseInt(stringValue, 10, 64)
//	if err != nil {
//		fmt.Println("Error converting string to int64:", err)
//		return 0
//	}
//
//	return val
//}

package pipeline

import (
	"bytes"
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/utils"
	"encoding/json"
	"errors"
	es7 "github.com/elastic/go-elasticsearch/v7"
	"go.uber.org/zap"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

var totalProcessed int64 = 0

func ImportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, redisClient *clients.RedisClient, transformedDocs <-chan map[string]interface{}, wg *sync.WaitGroup) {
	redisClient.Ctx = ctx

	//esClient, ok := client.(*clients.ES8Client)
	esClient, ok := client.(*clients.ES7Client)
	if !ok {
		logger.Error("Invalid client type; expected *ES8Client")
		return
	}

	// Check if the index exists
	_, err := esClient.Client.Indices.Exists([]string{config.Elk8.Index}, esClient.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		logger.Error("Error checking if index exists", zap.Error(err))
		return
	}

	bulkData := make([]map[string]interface{}, 0, config.App.BulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		if len(bulkData) >= config.App.BulkSize {
			wg.Add(1)
			go func(data []map[string]interface{}) {
				defer wg.Done()
				err := utils.Retry(ctx, config.App.MaxRetries, config.App.TTL, func() error {
					return sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, data, config.App.MaxBulkPayloadBytes)
				})
				if err != nil {
					logger.Error("Error sending bulk request", zap.Error(err))
				}
			}(bulkData)

			bulkData = make([]map[string]interface{}, 0, config.App.BulkSize) // Reset bulk buffer
		}
	}

	// Handle remaining documents
	if len(bulkData) > 0 {
		wg.Add(1)
		go func(data []map[string]interface{}) {
			defer wg.Done()
			if err := sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, data, config.App.MaxBulkPayloadBytes); err != nil {
				logger.Error("Error during final bulk insert", zap.Error(err))
			}
		}(bulkData)
	}

	wg.Wait() // Wait for all goroutines to finish
	logger.Info("All bulk insert operations completed.")
}

func sendBulkRequest(config *config.Config, client *es7.Client, redisClient *clients.RedisClient, index string, bulkData []map[string]interface{}, maxBulkPayloadBytes int) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	// Ensure the index is not empty
	if index == "" {
		return errors.New("index name is empty")
	}

	// Update totalProcessed count
	totalProcessed = updateTotalProcessed(redisClient, config.Redis)

	// Prepare bulk request
	for _, doc := range bulkData {
		atomic.AddInt64(&totalProcessed, 1)

		err := redisClient.Set(config.Redis.KeyTotalProcessed, totalProcessed, config.Redis.TTL)
		if err != nil {
			logger.Warn("Failed to save totalProcessed to Redis", zap.Error(err))
			return err
		}

		meta := map[string]interface{}{
			//TODO when streaming index => 'create' / not streaming 'index'
			//"create": map[string]interface{}{
			//	"_index": index,
			//},
			"create": map[string]interface{}{
				"_index": index,
			},
		}

		if err := encoder.Encode(meta); err != nil {
			logger.Error("Failed to encode metadata", zap.Error(err))
			return err
		}
		if err := encoder.Encode(doc); err != nil {
			logger.Error("Failed to encode document", zap.Error(err))
			return err
		}

		// Check if the payload size exceeds the limit
		if buf.Len() >= maxBulkPayloadBytes {
			if err := executeBulkRequest(client, buf.Bytes()); err != nil {
				return err
			}
			buf.Reset() // Reset buffer for the next batch
		}
	}

	// Send remaining documents
	if buf.Len() > 0 {
		if err := executeBulkRequest(client, buf.Bytes()); err != nil {
			logger.Warn("executeBulkRequest err", zap.Error(err))
			return err
		}
	}

	logger.Info("Bulk request completed", zap.Int64("total_documents_processed", totalProcessed))
	return nil
}

func executeBulkRequest(client *es7.Client, bulkPayload []byte) error {
	logger.Info("Bulk Request Payload", zap.String("payload", string(bulkPayload)))

	res, err := client.Bulk(bytes.NewReader(bulkPayload))
	if err != nil {
		logger.Error("Failed to execute bulk request", zap.Error(err))
		return err
	}
	defer res.Body.Close()

	// Read the response body
	body, _ := io.ReadAll(res.Body)
	logger.Info("Bulk Response from Elasticsearch", zap.String("response", string(body)))

	// Parse JSON response
	var bulkResponse map[string]interface{}
	if err := json.Unmarshal(body, &bulkResponse); err != nil {
		logger.Error("Failed to parse bulk response", zap.Error(err))
		return err
	}

	// Check if any errors occurred
	if err, exists := bulkResponse["errors"].(bool); exists && err {
		logger.Error("Bulk request partially failed", zap.String("response", string(body)))
		return errors.New("some documents failed in bulk insert")
	}

	logger.Info("Bulk request executed successfully")
	return nil
}

func updateTotalProcessed(redisClient *clients.RedisClient, redisConfig config.Redis) int64 {
	stringValue, err := redisClient.Get(redisConfig.KeyTotalProcessed)
	if err != nil {
		logger.Warn("Failed to retrieve totalProcessed from Redis", zap.Error(err))
		return 0
	}

	val, err := strconv.ParseInt(stringValue, 10, 64)
	if err != nil {
		logger.Error("Error converting string to int64", zap.Error(err))
		return 0
	}

	return val
}
