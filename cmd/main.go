package main

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/pipeline"
	"gopkg.in/olivere/elastic.v3"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/zap"
)

// Configuration for worker counts and buffer sizes
const (
	exportWorkers    = 4
	transformWorkers = 4
	importWorkers    = 4
	bufferSize       = 40000
)

func main() {
	// Record the start time
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Printf("Total execution time: %.2f seconds", duration.Seconds())
	}()

	logger.InitLogger("./logs/elkmigration.log")
	//logger.InitZLogger()
	defer logger.Log.Sync()

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Config Loading err, Set Default Values... ", zap.Error(err))
	}

	//utils.Generate(config)
	//return
	clients.InitRedis(logger.Log, cfg)
	defer clients.CloseRedis()

	// Get the number of available CPU cores
	numCPU := runtime.NumCPU()
	logger.Info("Available CPUs: %d\n", zap.Any("", numCPU))

	// Set the maximum number of CPUs to use
	runtime.GOMAXPROCS(numCPU) // Or set to a specific number like 4, depending on the need

	// Verify the number of CPUs Go is using
	logger.Info("Go is using %d CPUs\n", zap.Any("", runtime.GOMAXPROCS(0)))

	logger.Info("Starting Elasticsearch migration")

	// Initialize Elasticsearch clients
	es2Client, err := clients.NewElasticsearchClient(2, cfg.Elk2Url, cfg.Elk2User, cfg.Elk2Pass)
	if err != nil {
		logger.Error("Error creating Elasticsearch 2.x client", zap.Error(err))
		return
	}

	es8Client, err := clients.NewElasticsearchClient(8, cfg.Elk8Url, cfg.ELK8User, cfg.Elk8Pass)
	if err != nil {
		logger.Error("Error creating Elasticsearch 8.x client", zap.Error(err))
		return
	}

	// Channels for pipeline stages with buffer
	docs := make(chan *elastic.SearchResult, bufferSize)
	transformedDocs := make(chan map[string]interface{}, bufferSize)
	var ctx = context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Export stage worker pool
	for i := 0; i < exportWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			logger.Info("Starting export worker", zap.Int("workerID", workerID))
			pipeline.ExportDocuments(ctx, es2Client, cfg, docs, clients.RedisClient, &mu)
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
			pipeline.ImportDocuments(es8Client, cfg, transformedDocs)
			logger.Info("Import worker completed", zap.Int("workerID", workerID))
		}(i)
	}

	// Close channels after all work is done
	wg.Wait()
	//close(docs)            // Close docs to stop transformers
	//close(transformedDocs) // Close transformedDocs to stop importers

	logger.Info("Elasticsearch migration completed")
}
