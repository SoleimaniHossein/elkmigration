package main

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/pipeline"
	"fmt"
	"gopkg.in/olivere/elastic.v3"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Configuration for worker counts and buffer sizes
const (
	exportWorkers    = 1
	transformWorkers = 1
	importWorkers    = 1
)

func main() {
	start := time.Now()

	defer func() {
		duration := time.Since(start)
		logger.Info("Elasticsearch migration completed.", zap.Duration("Total Duration Time", duration))
	}()

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var wg sync.WaitGroup
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	cfg, err := config.LoadConfig()

	if err != nil {
		logger.Error("Config Loading err, Set Default Values... ", zap.Error(err))
	}

	logger.InitLogger()
	defer logger.Log.Sync()

	clients.InitRedis(ctx, &mu, cfg.Redis)
	defer clients.CloseRedis()

	// Get the number of available CPU cores
	numCPU := runtime.NumCPU()
	logger.Info("Available CPUs:", zap.Any("CPUs", numCPU))

	// Set the maximum number of CPUs to use
	runtime.GOMAXPROCS(numCPU) // Or set to a specific number like 4, depending on the need

	// Verify the number of CPUs Go is using
	logger.Info("Go is using ", zap.Any("CPUs", runtime.GOMAXPROCS(0)))

	logger.Info("Starting Elasticsearch migration...")

	// Initialize Elasticsearch clients
	es2Client, err := clients.NewElasticsearchClient(2, cfg.Elk2.Urls, cfg.Elk2.User, cfg.Elk2.Pass)
	if err != nil {
		logger.Error("Error creating Elasticsearch 2.x client", zap.Error(err))
		return
	}

	es8Client, err := clients.NewElasticsearchClient(8, cfg.Elk8.Urls, cfg.Elk8.User, cfg.Elk8.Pass)
	if err != nil {
		logger.Error("Error creating Elasticsearch 8.x client", zap.Error(err))
		return
	}

	docs := make(chan *elastic.SearchResult, cfg.App.BulkSize)
	transformedDocs := make(chan map[string]interface{}, cfg.App.BulkSize)

	for i := 0; i < exportWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pipeline.ExportDocuments(ctx, cfg, es2Client, docs, clients.RC)
		}(i)
	}

	for i := 0; i < transformWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pipeline.TransformDocuments(docs, transformedDocs, cfg.Elk2.SortBy)
		}(i)
	}

	ctxImport, _ := context.WithCancel(context.Background())
	for i := 0; i < importWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pipeline.ImportDocuments(ctxImport, cfg, es8Client, clients.RC, transformedDocs)
		}(i)
	}

	go func() {
		sig := <-signalChan
		fmt.Printf("Received signal: %s\n", sig)
		cancel()
		close(signalChan)
	}()

	wg.Wait()

}
