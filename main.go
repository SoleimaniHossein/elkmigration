package main

import (
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/pipeline"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/zap"
)

// Configuration for worker counts and buffer sizes
const (
	exportWorkers    = 20
	transformWorkers = 20
	importWorkers    = 20
	bufferSize       = 1000
)

func main() {
	// Record the start time
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Printf("Total execution time: %.2f seconds", duration.Seconds())
	}()

	logger.InitLogger()
	logger.InitZLogger()
	defer logger.Log.Sync()

	c, err := config.LoadConfig()
	if err != nil {
		logger.ZError("Config Loading err, Set Default Values... ", err)
	}

	clients.InitRedis(logger.Log, c)
	defer clients.CloseRedis()

	logger.ZInfo("Starting Elasticsearch migration")

	// Initialize Elasticsearch clients
	es2Client, err := clients.NewElasticsearchClient(2, c.Elk2URL)
	if err != nil {
		logger.Error("Error creating Elasticsearch 2.x client", zap.Error(err))
		return
	}

	es8Client, err := clients.NewElasticsearchClient(8, c.Elk8URL)
	if err != nil {
		logger.Error("Error creating Elasticsearch 8.x client", zap.Error(err))
		return
	}

	// Channels for pipeline stages with buffer
	docs := make(chan map[string]interface{}, bufferSize)
	transformedDocs := make(chan map[string]interface{}, bufferSize)
	var wg sync.WaitGroup
	var mu sync.Mutex // Mutex for shared resources

	// Export stage worker pool
	for i := 0; i < exportWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger.Info("Starting export worker", zap.Int("workerID", workerID))
			pipeline.ExportDocuments(es2Client, "oxygen", docs, clients.RedisClient, &mu)
			logger.Info("Export worker completed", zap.Int("workerID", workerID))
		}(i)
	}

	// Transform stage worker pool
	for i := 0; i < transformWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger.Info("Starting transform worker", zap.Int("workerID", workerID))
			pipeline.TransformDocuments(docs, transformedDocs)
			logger.Info("Transform worker completed", zap.Int("workerID", workerID))
		}(i)
	}

	// Import stage worker pool
	for i := 0; i < importWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger.Info("Starting import worker", zap.Int("workerID", workerID))
			pipeline.ImportDocuments(es8Client, "new-8-oxygen", transformedDocs)
			logger.Info("Import worker completed", zap.Int("workerID", workerID))
		}(i)
	}

	// Close channels after all work is done
	//go func() {
	wg.Wait()
	close(docs)            // Close docs to stop transformers
	close(transformedDocs) // Close transformedDocs to stop importers
	//}()

	logger.ZInfo("Elasticsearch migration completed")
}
