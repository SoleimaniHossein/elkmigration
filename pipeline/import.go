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
	"io"
	"strings"
	"sync/atomic"

	"github.com/dustin/go-humanize"
	es8 "github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

var (
	errorCount int64
)

func ImportDocuments(ctx context.Context, config *config.Config, elk8Client clients.ElasticsearchClient, transformedDocs <-chan map[string]interface{}) {

	es8Client, ok := elk8Client.(*clients.Es8Client)
	if !ok {
		logger.Fatal("Invalid client type; expected *ES8Client")
	}

	// Check if the index exists
	_, err := es8Client.Client.Indices.Exists([]string{config.Elk8.Index}, es8Client.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		logger.Fatal("Error checking if index exists", zap.Error(err))
	}

	bulkData := make([]map[string]interface{}, 0, config.App.BulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		if len(bulkData) >= config.App.BulkSize {
			if handleBulkInsert(ctx, config, es8Client.Client, &bulkData) {
				return // Exit if max errors reached
			}
		}
	}

	// Handle remaining documents
	if len(bulkData) > 0 {
		handleBulkInsert(ctx, config, es8Client.Client, &bulkData)
	}
}

func handleBulkInsert(ctx context.Context, config *config.Config, client *es8.Client, bulkData *[]map[string]interface{}) bool {
	if len(*bulkData) == 0 {
		return false
	}

	err := utils.Retry(ctx, config.App.MaxRetries, config.App.TTL, func() error {
		return sendBulkRequest(ctx, client, config.Elk8.Index, *bulkData, config.App.MaxBulkPayloadBytes)
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

func sendBulkRequest(ctx context.Context, client *es8.Client, index string, bulkData []map[string]interface{}, maxBulkPayloadBytes int) error {
	if index == "" {
		return errors.New("index name is empty")
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	//totalProcessed = updateTotalProcessed()

	for _, doc := range bulkData {

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

	count, err := GetDocumentCount(ctx, client)
	if err != nil {
		logger.Error("Error getting document count", zap.Error(err))
		return err
	}

	logger.Info("Bulk request completed", zap.String("total_documents_processed", humanize.Comma(count)))
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

	// Log raw response if request fails
	if res.IsError() {
		logger.Error("Elasticsearch HTTP error", zap.String("status", res.Status()), zap.String("body", string(body)))

		// Handle payload too large (413)
		if res.StatusCode == 413 {
			logger.Fatal("Bulk payload is too large! Reduce 'maxBulkPayloadBytes'")
		}

		return fmt.Errorf("elasticsearch error: %s", res.Status())
	}

	// Parse the bulk response
	var bulkResponse struct {
		Errors bool                                `json:"errors"`
		Items  []map[string]map[string]interface{} `json:"items"`
	}

	if err := json.Unmarshal(body, &bulkResponse); err != nil {
		logger.Error("Failed to parse bulk response", zap.Error(err))
		return err
	}

	// Check for document-specific errors
	if bulkResponse.Errors {
		errorCount := 0
		for _, item := range bulkResponse.Items {
			for action, result := range item {
				if errMsg, ok := result["error"].(map[string]interface{}); ok {
					docID, _ := result["_id"].(string) // Get document ID if available
					logger.Error(fmt.Sprintf("Failed to %s document", action),
						zap.String("doc_id", docID),
						zap.Any("error", errMsg),
					)
					errorCount++
				}
			}
		}
		logger.Warn("Bulk request had document-level errors", zap.Int("error_count", errorCount))
	}

	return nil
}

// CountResponse represents the response structure of the _count API.
type CountResponse struct {
	Count int64 `json:"count"`
}

// GetDocumentCount retrieves the number of documents in an Elasticsearch 8 index.
func GetDocumentCount(ctx context.Context, es8Client *es8.Client) (int64, error) {
	// Prepare the request body (optional query filter)
	body := `{"query": {"match_all": {}}}`

	// Execute the count request
	res, err := es8Client.Count(
		es8Client.Count.WithContext(ctx),
		es8Client.Count.WithBody(strings.NewReader(body)),
		es8Client.Count.WithPretty(),
	)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	// Parse the response
	var countResp CountResponse
	if err := json.NewDecoder(res.Body).Decode(&countResp); err != nil {
		return 0, err
	}

	return countResp.Count, nil
}
