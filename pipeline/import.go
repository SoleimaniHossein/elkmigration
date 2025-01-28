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
	"fmt"
	es8 "github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
	"strconv"
	"sync/atomic"
)

var totalProcessed int64 = 0

func ImportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, redisClient *clients.RedisClient, transformedDocs <-chan map[string]interface{}) {
	redisClient.Ctx = ctx

	esClient, ok := client.(*clients.ES8Client) // Type assertion for ES8Client

	if !ok {
		logger.Error("Invalid client type; expected *ES8Client")
		return
	}

	_, err := esClient.Client.Indices.Exists([]string{config.Elk8.Index}, esClient.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		logger.Error("Error checking if index exists", zap.Error(err))
		return
	}

	bulkData := make([]map[string]interface{}, 0, config.App.BulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		// Send bulk request when reaching the bulkSize
		if len(bulkData) >= config.App.BulkSize {

			err = utils.Retry(ctx, config.App.MaxRetries, config.App.TTL, func() error {
				return sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, bulkData, config.App.MaxBulkPayloadBytes)
			})
			if err != nil {
				logger.Error("Error sending bulk request", zap.Error(err))
			}
			bulkData = bulkData[:0] // Reset the bulk data buffer
		}
	}

	// Send any remaining documents
	if len(bulkData) > 0 {
		if err := sendBulkRequest(config, esClient.Client, redisClient, config.Elk8.Index, bulkData, config.App.MaxBulkPayloadBytes); err != nil {
			logger.Error("Error during final bulk insert", zap.Error(err))
		}
	}
}

func sendBulkRequest(config *config.Config, client *es8.Client, redisClient *clients.RedisClient, index string, bulkData []map[string]interface{}, maxBulkPayloadBytes int) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	totalProcessed = updateTotalProcessed(redisClient, config.Redis)
	// Prepare bulk request format
	for _, doc := range bulkData {

		atomic.AddInt64(&totalProcessed, 1)

		err := redisClient.Set(config.Redis.KeyTotalProcessed, totalProcessed, config.Redis.TTL)

		if err != nil {
			logger.Warn("failed to save totalProcessed to Redis", zap.Error(err))
			return err
		}

		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
			},
		}
		if err := encoder.Encode(meta); err != nil {
			return err
		}
		if err := encoder.Encode(doc); err != nil {
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
			logger.Warn("executeBulkRequest err: %s", zap.Error(err))
		}
	}

	logger.Info("Bulk request completed", zap.Int64("documents_count", totalProcessed))

	return nil
}

func executeBulkRequest(client *es8.Client, bulkPayload []byte) error {
	res, err := client.Bulk(bytes.NewReader(bulkPayload))
	if err != nil {
		logger.Error("Failed to execute bulk request", zap.Error(err))
		return err
	}
	defer res.Body.Close()

	// Check for errors in the response
	if res.IsError() {
		logger.Error("Bulk request failed when importing", zap.String("status", res.Status()))
		return errors.New("bulk request failed")
	}
	return nil
}

func updateTotalProcessed(redisClient *clients.RedisClient, redisConfig config.Redis) int64 {
	stringValue, err := redisClient.Get(redisConfig.KeyTotalProcessed)

	if err != nil {
		logger.Warn("not found total processed", zap.Error(err))
		return 0
	}

	val, err := strconv.ParseInt(stringValue, 10, 64)
	if err != nil {
		fmt.Println("Error converting string to int64:", err)
		return 0
	}

	return val
}
