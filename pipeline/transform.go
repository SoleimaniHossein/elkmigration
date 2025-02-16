package pipeline

import (
	"elkmigration/logger"
	"encoding/json"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
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
					doc["@timestamp"] = val
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
