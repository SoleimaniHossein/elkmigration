package pipeline

import (
	"bytes"
	"elkmigration/clients"
	"elkmigration/logger"
	"encoding/json"
	"log"
	"time"

	//"github.com/elastic/go-elasticsearch/v7"
	es8 "github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

func ImportDocuments(client clients.ElasticsearchClient, index string, transformedDocs <-chan map[string]interface{}) {
	//es7Client, ok7 := client.(*clients.ES7Client) // Type assertion
	esClient, ok := client.(*clients.ES8Client) // Type assertion
	if !ok {
		logger.Error("Invalid client type; expected *ES8Client")
		return
	}

	bulkData := make([]map[string]interface{}, 0, bulkSize)

	for doc := range transformedDocs {
		bulkData = append(bulkData, doc)

		// Send batch when reaching the bulkSize
		if len(bulkData) >= bulkSize {
			if err := sendBulkRequest(esClient.Client, index, bulkData); err != nil {
				log.Printf("Error during bulk insert: %v", err)
				time.Sleep(1 * time.Second) // Wait before retrying
			}
			bulkData = bulkData[:0] // Clear the slice for the next batch
		}
	}

	// Send any remaining documents
	if len(bulkData) > 0 {
		if err := sendBulkRequest(esClient.Client, index, bulkData); err != nil {
			log.Printf("Error during final bulk insert: %v", err)
		}
	}
}

// sendBulkRequest: Sends a batch of documents to Elasticsearch
func sendBulkRequest(client *es8.Client, index string, bulkData []map[string]interface{}) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	// Prepare bulk request format for Elasticsearch
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
	}

	// Perform bulk request
	res, err := client.Bulk(bytes.NewReader(buf.Bytes()))
	if err != nil {
		logger.Error("Failed to execute bulk request", zap.Error(err))
		return err
	}
	defer res.Body.Close()

	// Check for bulk request errors
	if res.IsError() {
		logger.Error("Bulk request failed", zap.String("status", res.Status()))
		return err
	}

	logger.Info("Successfully imported bulk documents", zap.Int("documents_count", len(bulkData)))
	return nil
}
