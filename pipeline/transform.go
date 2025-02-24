package pipeline

import (
	"elkmigration/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
	"time"
)

func TransformDocuments(result <-chan *elastic.SearchResult, transformedDocs chan<- map[string]interface{}, timeStampField string) {
	defer close(transformedDocs)

	for res := range result {
		for _, hit := range res.Hits.Hits {
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				logger.Warn("Error unmarshalling document", zap.Error(err))
				continue
			}

			// Handle timestamp conversion
			if _, exists := doc["@timestamp"]; !exists {
				var timestamp time.Time
				switch v := doc[timeStampField].(type) {
				case float64:
					timestamp = time.Unix(int64(v)/1e3, (int64(v)%1e3)*1e6).UTC()
				case int64:
					timestamp = time.Unix(v/1e3, (v%1e3)*1e6).UTC()
				case string:
					parsedTime, err := time.Parse(time.RFC3339, v)
					if err != nil {
						logger.Warn("Invalid timestamp format", zap.String("value", v))
						timestamp = time.Now().UTC() // Fallback to current time
					} else {
						timestamp = parsedTime.UTC()
					}
				case time.Time:
					timestamp = v.UTC()
				default:
					if dtVal, ok := doc["datetime"]; ok {
						if parsedTime, err := time.Parse(time.RFC3339, fmt.Sprintf("%v", dtVal)); err == nil {
							timestamp = parsedTime.UTC()
						} else {
							logger.Warn("Invalid datetime format", zap.Any("value", dtVal))
							timestamp = time.Now().UTC() // Fallback
						}
					} else {
						logger.Warn("Timestamp field missing, setting default")
						timestamp = time.Now().UTC() // Default if no timestamp found
					}
				}

				// Ensure valid year range for JSON encoding
				if timestamp.Year() < 0 || timestamp.Year() > 9999 {
					logger.Warn("Timestamp out of valid range", zap.Time("timestamp", timestamp))
					timestamp = time.Now().UTC() // Set to a valid time
				}

				// Store as ISO 8601 formatted string for Elasticsearch compatibility
				doc["@timestamp"] = timestamp.Format(time.RFC3339)
			}

			transformedDocs <- doc
		}
	}
}
