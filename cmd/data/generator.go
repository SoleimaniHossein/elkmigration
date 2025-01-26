package main

import (
	"bytes"
	"elkmigration/config"
	"elkmigration/logger"
	"encoding/json"
	"fmt"
	"github.com/bxcodec/faker/v3"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// FakeDocument represents a single fake JSON document.
type FakeDocument struct {
	ID           string   `json:"uuid"`
	Counter      int      `faker:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	PhoneNumber  string   `json:"phone_number"`
	BirthDate    string   `json:"birth_date"`
	RegisteredAt string   `json:"registered_at"`
	Status       string   `json:"status"`
	Score        int      `json:"score"`
	Tags         []string `json:"tags"`
}

type FakeUser struct {
	ID         string `json:"uuid"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	UserNumber int    `json:"user_number"`
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

	logger.InitLogger(cfg.LogPath)
	defer logger.Log.Sync()

	generate(cfg, 100)

	return
}

// GenerateFakeDocument creates a single fake document.
func generateFakeDocument(counter int) FakeDocument {
	statusOptions := []string{"active", "inactive", "pending"}
	tagsOptions := []string{"tag1", "tag2", "tag3", "tag4", "tag5"}

	return FakeDocument{
		ID:           faker.UUIDDigit(),
		Counter:      generateSequentialID(counter),
		Name:         faker.Name(),
		Email:        faker.Email(),
		PhoneNumber:  faker.Phonenumber(),
		BirthDate:    faker.Date(),
		RegisteredAt: time.Now().Format(time.RFC3339),
		Status:       statusOptions[rand.Intn(len(statusOptions))],
		Score:        rand.Intn(101),
		Tags:         []string{tagsOptions[rand.Intn(len(tagsOptions))], tagsOptions[rand.Intn(len(tagsOptions))]},
	}
}

// GenerateFakeUsersDoc creates a single fake document.
func generateFakeUsersDoc(counter int) FakeUser {
	return FakeUser{
		ID:         faker.UUIDDigit(),
		Name:       fmt.Sprintf("User%d", counter),
		Email:      faker.Email(),
		UserNumber: counter,
	}
}

// Generate populates Elasticsearch with fake documents.
func generate(config *config.Config, numRecords int) {
	docType := "document"
	bulkURL := fmt.Sprintf("%s/%s/%s/_bulk", config.Elk2Url, config.ElkIndexFrom, docType)

	client := &http.Client{Timeout: config.TTL}
	var bulkBuffer bytes.Buffer

	for i := 1; i <= numRecords; i++ {
		doc := generateFakeUsersDoc(i)

		// Add metadata line
		meta := fmt.Sprintf(`{ "index" : { "_id" : "%s" } }`, doc.ID)
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
		if (i)%config.BulkSize == 0 || i == numRecords {
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

func generateSequentialID(counter int) int {
	var idCounter int32
	idCounter = int32(counter)
	return int(atomic.AddInt32(&idCounter, 1))
}
