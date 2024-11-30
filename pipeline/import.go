package pipeline

import (
	"bytes"
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"encoding/json"
	"errors"
	"time"

	es8 "github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

const (
	retryDelay          = 1 * time.Second   // Delay between retries on failure
	maxBulkPayloadBytes = 100 * 1024 * 1024 // Set a 100MB limit per bulk request
)

// ImportDocuments imports documents into Elasticsearch.
func ImportDocuments(client clients.ElasticsearchClient, config *config.Config, transformedDocs <-chan map[string]interface{}) {
	esClient, ok := client.(*clients.ES8Client) // Type assertion for ES8Client

	if !ok {
		logger.Error("Invalid client type; expected *ES8Client")
		return
	}

	// Check if the target index exists
	ctx := context.Background()
	_, err := esClient.Client.Indices.Exists([]string{config.ElkIndexTo}, esClient.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		logger.Error("Error checking if index exists", zap.Error(err))
		return
	}

	bulkData := make([]map[string]interface{}, 0, config.BulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		// Send bulk request when reaching the bulkSize
		if len(bulkData) >= config.BulkSize {
			if err := sendBulkRequest(esClient.Client, config.ElkIndexTo, bulkData); err != nil {
				logger.Warn("Error during bulk insert, retrying...", zap.Error(err))
				time.Sleep(retryDelay)
			}
			bulkData = bulkData[:0] // Reset the bulk data buffer
		}
	}

	// Send any remaining documents
	if len(bulkData) > 0 {
		if err := sendBulkRequest(esClient.Client, config.ElkIndexTo, bulkData); err != nil {
			logger.Error("Error during final bulk insert", zap.Error(err))
		}
	}
}

func sendBulkRequest(client *es8.Client, index string, bulkData []map[string]interface{}) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	// Prepare bulk request format
	for _, doc := range bulkData {
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

	logger.Info("Bulk request completed", zap.Int("documents_count", len(bulkData)))
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
