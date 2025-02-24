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
	"io"
	"strconv"
	"sync/atomic"
)

var (
	totalProcessed int64
	errorCount     int64
)

func ImportDocuments(ctx context.Context, config *config.Config, client clients.ElasticsearchClient, redisClient *clients.RedisClient, transformedDocs <-chan map[string]interface{}) {
	redisClient.Ctx = ctx

	esClient, ok := client.(*clients.Es8Client)
	if !ok {
		logger.Fatal("Invalid client type; expected *ES8Client")
	}

	// Check if the index exists
	_, err := esClient.Client.Indices.Exists([]string{config.Elk8.Index}, esClient.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		logger.Fatal("Error checking if index exists", zap.Error(err))
	}

	bulkData := make([]map[string]interface{}, 0, config.App.BulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		if len(bulkData) >= config.App.BulkSize {
			if handleBulkInsert(ctx, config, esClient.Client, redisClient, &bulkData) {
				return // Exit if max errors reached
			}
		}
	}

	// Handle remaining documents
	if len(bulkData) > 0 {
		handleBulkInsert(ctx, config, esClient.Client, redisClient, &bulkData)
	}
}

func handleBulkInsert(ctx context.Context, config *config.Config, client *es8.Client, redisClient *clients.RedisClient, bulkData *[]map[string]interface{}) bool {
	if len(*bulkData) == 0 {
		return false
	}

	err := utils.Retry(ctx, config.App.MaxRetries, config.App.TTL, func() error {
		return sendBulkRequest(config, client, redisClient, config.Elk8.Index, *bulkData, config.App.MaxBulkPayloadBytes)
	})

	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		logger.Error("Error sending bulk request", zap.Error(err), zap.Int64("error_count", atomic.LoadInt64(&errorCount)))

		if atomic.LoadInt64(&errorCount) >= 3 {
			logger.Fatal("Too many bulk request failures. Exiting...")
			return true
		}
	}

	// Reset bulkData to free memory
	*bulkData = (*bulkData)[:0]

	return false
}

func sendBulkRequest(config *config.Config, client *es8.Client, redisClient *clients.RedisClient, index string, bulkData []map[string]interface{}, maxBulkPayloadBytes int) error {
	if index == "" {
		return errors.New("index name is empty")
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	totalProcessed = updateTotalProcessed(redisClient, config.Redis)

	for _, doc := range bulkData {
		atomic.AddInt64(&totalProcessed, 1)
		err := redisClient.Set(config.Redis.KeyTotalProcessed, totalProcessed, config.Redis.TTL)
		if err != nil {
			logger.Warn("Failed to save totalProcessed to Redis", zap.Error(err))
		}

		meta := map[string]interface{}{"create": map[string]interface{}{"_index": index}}
		if err := encoder.Encode(meta); err != nil {
			logger.Error("Failed to encode metadata", zap.Error(err))
			return err
		}
		if err := encoder.Encode(doc); err != nil {
			logger.Error("Failed to encode document", zap.Error(err), zap.Any("doc", doc))
			return err
		}

		// Check payload size limit
		if buf.Len() >= maxBulkPayloadBytes {
			if err := executeBulkRequest(client, buf.Bytes()); err != nil {
				return err
			}
			buf.Reset()
		}
	}

	// Send remaining documents
	if buf.Len() > 0 {
		if err := executeBulkRequest(client, buf.Bytes()); err != nil {
			logger.Warn("executeBulkRequest error", zap.Error(err))
			return err
		}
	}

	logger.Info("Bulk request completed", zap.Int64("total_documents_processed", atomic.LoadInt64(&totalProcessed)))
	return nil
}

func executeBulkRequest(client *es8.Client, bulkPayload []byte) error {
	res, err := client.Bulk(bytes.NewReader(bulkPayload))
	if err != nil {
		logger.Error("Failed to execute bulk request", zap.Error(err))
		return err
	}
	defer res.Body.Close()

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		logger.Error("Failed to read bulk response", zap.Error(readErr))
		return readErr
	}

	if res.IsError() {
		logger.Error("Elasticsearch HTTP error", zap.String("status", res.Status()), zap.String("body", string(body)))
		return fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	var bulkResponse struct {
		Errors bool                                `json:"errors"`
		Items  []map[string]map[string]interface{} `json:"items"`
	}

	if err := json.Unmarshal(body, &bulkResponse); err != nil {
		logger.Error("Failed to parse bulk response", zap.Error(err))
		return err
	}

	if bulkResponse.Errors {
		for _, item := range bulkResponse.Items {
			for action, result := range item {
				if errMsg, ok := result["error"].(map[string]interface{}); ok {
					logger.Error(fmt.Sprintf("%s: %v", action, errMsg))
				}
			}
		}
	}

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
		logger.Warn("Error converting string to int64", zap.Error(err))
		return 0
	}

	return val
}
