package pipeline

import (
	"context"
	"elkmigration/clients"
	"elkmigration/config"
	"elkmigration/logger"
	"elkmigration/utils"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
	"io"
	"log"
)

// ExportDocuments exports documents from Elasticsearch 2.x using a range query and scroll API,
// continuing from the latest `started_at` timestamp in ELK 8.
func ExportDocuments(ctx context.Context, config *config.Config, elk2client clients.ElasticsearchClient, elk8Client clients.ElasticsearchClient, docs chan<- *elastic.SearchResult) {
	defer close(docs)

	// Step 1: Fetch the latest `started_at` from ELK 8
	latestStartedAt, err := GetLatestStartedAt(ctx, config, elk8Client)
	if err != nil {
		logger.Warn("Failed to get latest started_at from ELK 8. Defaulting to configured start date.", zap.Error(err))
		latestStartedAt = 0 // Fallback to the configured start date
	}

	logger.Info("Using latest started_at for migration", zap.Int64("latest_started_at", latestStartedAt))

	// Step 3: Construct Range Query (fetch documents from ELK 2 starting from `latestStartedAt`)
	rangeQuery := elastic.NewRangeQuery(config.Elk2.SortBy).
		Gt(latestStartedAt)

	es2Client := elk2client.(*clients.Es2Client).Client

	count, err := es2Client.Count(config.Elk2.Index).Query(rangeQuery).Do()

	if err != nil {
		log.Fatalf("Error getting count: %v", err)
	}

	logger.Info("Total documents to migrate", zap.Int64("count", count))

	// Step 4: Initialize Scroll Service
	var scrollService *elastic.ScrollService
	scrollService = es2Client.Scroll(config.Elk2.Index).
		Size(config.App.BulkSize).
		Query(rangeQuery).
		Sort(config.Elk2.SortBy, config.Elk2.Asc).
		Scroll(config.App.ScrollTTL)

	// Step 3: Fetch First Scroll Page with Retries
	result, err := utils.RetryWithResult(ctx, config.App.MaxRetries, config.App.TTL, func() (*elastic.SearchResult, error) {
		return scrollService.Do()
	})

	if err != nil {
		logger.Error("Failed to start Scroll API", zap.Error(err))
		return
	}

	scrollID := result.ScrollId // Initialize scrollID for next iterations

	// Step 4: Iterate and fetch documents in batches with retries
	for {
		if len(result.Hits.Hits) == 0 {
			logger.Info("Reached end of index")
			return
		}

		// Send batch to channel
		docs <- result

		// Fetch next batch using updated scrollID with retries
		result, err = utils.RetryWithResult(ctx, config.App.MaxRetries, config.App.TTL, func() (*elastic.SearchResult, error) {
			return es2Client.Scroll(config.App.ScrollTTL).
				ScrollId(scrollID).Do()
		})

		if errors.Is(err, elastic.EOS) {
			logger.Info("No more documents to export.", zap.Error(err))
			return
		}

		if err != nil {
			logger.Error("Scroll API error", zap.Error(err))
			return
		}

		// Update scrollID for the next loop
		scrollID = result.ScrollId
	}
}

// GetLatestStartedAt retrieves the latest `started_at` timestamp from Elasticsearch 8.
func GetLatestStartedAt(ctx context.Context, config *config.Config, elk8Client clients.ElasticsearchClient) (int64, error) {
	es8Client := elk8Client.(*clients.Es8Client).Client

	logger.Info(fmt.Sprintf("%s:%s", config.Elk8.SortBy, config.Elk8.OrderBy))
	// Elasticsearch query to fetch the latest started_at
	res, err := es8Client.Search(
		es8Client.Search.WithContext(ctx),
		es8Client.Search.WithIndex(config.Elk8.Index),
		es8Client.Search.WithPretty(),
		es8Client.Search.WithSort(fmt.Sprintf("%s:%s", config.Elk8.SortBy, config.Elk8.OrderBy)), // Get the latest first
		es8Client.Search.WithSourceIncludes(config.Elk8.SortBy),                                  // Fetch only `started_at`
		es8Client.Search.WithSize(1),
	)

	if err != nil {
		return 0, fmt.Errorf("elasticsearch search failed: %w", err)
	}
	defer res.Body.Close()

	// Check if Elasticsearch returned an error
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return 0, fmt.Errorf("elasticsearch error: %s", body)
	}

	// Define structure to parse the response JSON
	var searchResult struct {
		Hits struct {
			Hits []struct {
				Source struct {
					StartedAt int64 `json:"started_at"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	// Decode the JSON response
	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return 0, fmt.Errorf("failed to decode Elasticsearch response: %w", err)
	}

	// Check if we got any results
	if len(searchResult.Hits.Hits) == 0 {
		return 0, fmt.Errorf("no documents found in index: %s", config.Elk8.Index)
	}

	// Extract the latest `started_at` timestamp
	latestStartedAt := searchResult.Hits.Hits[0].Source.StartedAt
	log.Printf("Latest started_at fetched: %d", latestStartedAt)

	return latestStartedAt, nil
}
