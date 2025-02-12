package pipeline

import (
	"elkmigration/logger"
	"encoding/json"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v3"
)

func TransformDocuments(result <-chan *elastic.SearchResult, transformedDocs chan<- map[string]interface{}) {
	defer close(transformedDocs)
	for res := range result {
		for _, hit := range res.Hits.Hits {
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				logger.Warn("Error unmarshalling document", zap.Error(err))
				continue
			}

			if val, ok := doc["started_at"]; ok {
				doc["@timestamp"] = val
			}
			transformedDocs <- doc
		}
	}
}

// Example transformation: renaming fields
//if val, ok := doc["old_field"]; ok {
//	doc["new_field"] = val
//	delete(doc, "old_field")
//}

// Check if "id" exists and is a string before logging
//if id, ok := doc["id"].(string); ok && id != "" {
//	logger.Info("Transformed document", zap.String("doc_id", id))
//} else {
//	fmt.Println("=======================================================================================================")
//	logger.Warn("Document ID is missing or not a string in Process Transforming", zap.Any("document", doc))
//	fmt.Println("=======================================================================================================")
//	fmt.Println(reflect.TypeOf(doc))
//	fmt.Println(reflect.TypeOf(doc["id"]))
//
//	os.Exit(9)
//}

// Send transformed document to next stage
//transformedDocs <- doc
//logger.Info("Transformed document", zap.Any("client_ip", doc["client_ip"]))

//}
//}
