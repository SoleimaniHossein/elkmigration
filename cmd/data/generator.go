package main

import (
	"bytes"
	"elkmigration/config"
	"elkmigration/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"log"
	"net/http"
	"time"
)

type FakeUser struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	TimeStamp time.Time `json:"started_at"`
}

func main() {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		logger.Info("Elasticsearch Data Entry completed.", zap.Duration("Total Duration Time", duration))
	}()

	cfg, err := config.LoadConfig()

	if err != nil {
		logger.Error("Config Loading err, Set Default Values... ", zap.Error(err))
	}

	logger.InitLogger(cfg.App.LogPath)
	defer logger.Log.Sync()

	generate(cfg, 1_000_000)

	return
}

// GenerateFakeUsersDoc creates a single fake document.
func generateFakeUsersDoc(counter int) FakeUser {
	return FakeUser{
		ID:        counter,
		Name:      fmt.Sprintf("User%d", counter),
		Email:     fmt.Sprintf("email@user%d.com", counter),
		TimeStamp: time.Now(),
	}
}

// Generate populates Elasticsearch with fake documents.
func generate(config *config.Config, numRecords int) {
	docType := "document"
	bulkURL := fmt.Sprintf("%s/%s/%s/_bulk", config.Elk2.Url, config.Elk2.Index, docType)

	client := &http.Client{Timeout: config.App.TTL}
	var bulkBuffer bytes.Buffer

	for i := 1; i <= numRecords; i++ {
		doc := generateFakeUsersDoc(i)

		// Add metadata line
		meta := fmt.Sprintf(`{ "index" : { "_id" : "%d" } }`, doc.ID)
		bulkBuffer.WriteString(meta + "\n")

		// Add document JSON
		docJSON, err := json.Marshal(doc)
		if err != nil {
			log.Printf("Error marshalling document: %v", err)
			continue
		}
		bulkBuffer.Write(docJSON)
		bulkBuffer.WriteString("\n")

		// Send batch
		if (i)%config.App.BulkSize == 0 || i == numRecords {
			resp, err := client.Post(bulkURL, "application/json", &bulkBuffer)
			if err != nil {
				log.Printf("Error sending bulk request: %v", err)
				bulkBuffer.Reset() // Reset buffer for retry
				continue
			}

			if resp.StatusCode >= 400 {
				log.Printf("Bulk request failed with status %d", resp.StatusCode)
			} else {
				fmt.Printf("Successfully indexed %d documents\n", i)
			}

			resp.Body.Close()  // Close response body
			bulkBuffer.Reset() // Reset buffer after batch
		}
	}

	fmt.Println("Data generation and indexing complete.")
}
