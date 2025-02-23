package pipeline

import (
	"elkmigration/logger"
	"encoding/json"
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

			if _, ok := doc["@timestamp"]; !ok {
				if val, ok := doc[timeStampField]; ok {
					timestamp := int64(val.(float64))
					doc["@timestamp"] = time.Unix(timestamp/1e3, (timestamp%1e3)*1e6).UTC()
				} else if dtVal, ok := doc["datetime"]; ok {
					doc["@timestamp"] = dtVal
				} else {
					doc["@timestamp"] = 0
				}
			}

			transformedDocs <- doc
		}
	}
}
