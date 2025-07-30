package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// --- 입력 정보 (Command-line flags) ---
	sourceURI := flag.String("source.uri", "mongodb://localhost:27017", "Source MongoDB connection URI")
	targetURI := flag.String("target.uri", "mongodb://localhost:27017", "Target MongoDB connection URI")
	sourceDBName := flag.String("source.db", "source-db", "Source database name")
	targetDBName := flag.String("target.db", "target-db", "Target database name")
	sourceFilterStr := flag.String("source.filter", "{}", "Source collection filter as a JSON string")
	targetFilterStr := flag.String("target.filter", "{}", "Target collection filter as a JSON string")
	hideMatching := flag.Bool("hide-matching", false, "Hide matching indexes and counts from the output")
	forceCreateIndex := flag.Bool("force-create-index", false, "Force create index on target if mismatch or not exists")

	flag.Parse()
	// ------------------------------------

	// Parse filter strings into bson.M
	var sourceFilter, targetFilter bson.M
	if err := bson.UnmarshalExtJSON([]byte(*sourceFilterStr), true, &sourceFilter); err != nil {
		log.Fatalf("Failed to parse source filter: %v", err)
	}
	if err := bson.UnmarshalExtJSON([]byte(*targetFilterStr), true, &targetFilter); err != nil {
		log.Fatalf("Failed to parse target filter: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // 컨텍스트 시간 연장
	defer cancel()

	// Source Client 연결
	//fmt.Printf("Connecting to source MongoDB at %s...\n", *sourceURI)
	sourceClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*sourceURI))
	if err != nil {
		log.Fatalf("Failed to connect to source MongoDB: %v", err)
	}
	if err := sourceClient.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping source MongoDB: %v", err)
	}
	defer sourceClient.Disconnect(ctx)
	//fmt.Println("Successfully connected and pinged Source MongoDB.")

	// Target Client 연결
	//fmt.Printf("Connecting to target MongoDB at %s...\n", *targetURI)
	targetClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*targetURI))
	if err != nil {
		log.Fatalf("Failed to connect to target MongoDB: %v", err)
	}
	if err := targetClient.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping target MongoDB: %v", err)
	}
	defer targetClient.Disconnect(ctx)
	//fmt.Println("Successfully connected and pinged Target MongoDB.")

	sourceDB := sourceClient.Database(*sourceDBName)
	targetDB := targetClient.Database(*targetDBName)

	// Target DB의 모든 Collection 목록 가져오기
	fmt.Printf("Fetching collections from target database '%s'...\n", *targetDBName)
	collections, err := sourceDB.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		log.Fatalf("Failed to list collections from target DB '%s': %v", *targetDBName, err)
	}

	fmt.Printf("\n--- Comparison Details ---\n")
	fmt.Printf("Source DB: %s (Filter: %s) | Target DB: %s (Filter: %s)\n", *sourceDBName, *sourceFilterStr, *targetDBName, *targetFilterStr)

	// 각 Collection 순회하며 인덱스 및 건수 비교
	for _, collName := range collections {
		fmt.Printf("\nCollection: %s\n", collName)

		// 건수 비교
		sourceCount, err := sourceDB.Collection(collName).CountDocuments(ctx, sourceFilter)
		if err != nil {
			log.Printf("WARN: Failed to count documents in source collection '%s': %v", collName, err)
		}
		targetCount, err := targetDB.Collection(collName).CountDocuments(ctx, targetFilter)
		if err != nil {
			log.Printf("WARN: Failed to count documents in target collection '%s': %v", collName, err)
		}

		countMatch := "Match"
		if sourceCount != targetCount {
			countMatch = "Mismatch"
		}

		if !*hideMatching || countMatch == "Mismatch" {
			fmt.Printf("  - Document Count | Match: %s (Source: %d, Target: %d)\n", countMatch, sourceCount, targetCount)
		}

		// 인덱스 비교
		sourceIndexMap := getIndexMap(ctx, sourceDB, collName)
		targetIndexMap := getIndexMap(ctx, targetDB, collName)

		// 인덱스 비교 및 결과 출력
		for name, targetIndex := range targetIndexMap {
			if sourceIndex, exists := sourceIndexMap[name]; exists {
				reasons := compareIndexes(sourceIndex, targetIndex)
				if len(reasons) == 0 {
					if !*hideMatching {
						fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Match")
					}
				} else {
					reasonStr := strings.Join(reasons, ", ")
					fmt.Printf("  - Index: %-30s | Match: %s (%s)\n", name, "Mismatch", reasonStr)
					if *forceCreateIndex {
						dropIndex(ctx, targetDB, collName, name)
						createIndexFromModel(ctx, targetDB, collName, sourceIndex)
					}
				}
				delete(sourceIndexMap, name)
			} else {
				fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Mismatch (Not in Source)")
				if *forceCreateIndex {
					dropIndex(ctx, targetDB, collName, name)
				}
			}
		}

		for name, sourceIndex := range sourceIndexMap {
			fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Mismatch (Not in Target)")
			// 인덱스 생성 구문 생성
			createIndexStatement := generateCreateIndexStatement(collName, sourceIndex)
			fmt.Printf("    - Create Index Statement: %s\n", createIndexStatement)
			if *forceCreateIndex {
				createIndexFromModel(ctx, targetDB, collName, sourceIndex)
			}
		}
	}
}

func dropIndex(ctx context.Context, db *mongo.Database, collName string, indexName string) {
	_, err := db.Collection(collName).Indexes().DropOne(ctx, indexName)
	if err != nil {
		log.Printf("WARN: Failed to drop index '%s' on collection '%s': %v", indexName, collName, err)
	} else {
		fmt.Printf("    - Dropped index '%s' from target collection '%s'\n", indexName, collName)
	}
}

func createIndexFromModel(ctx context.Context, db *mongo.Database, collName string, indexData bson.D) {
	indexDataMap := indexData.Map()

	keys, ok := indexDataMap["key"]
	if !ok {
		log.Printf("WARN: Could not determine index keys for collection '%s'", collName)
		return
	}

	indexModel := mongo.IndexModel{
		Keys:    keys,
		Options: options.Index(),
	}

	indexName := "unknown"
	if name, ok := indexDataMap["name"].(string); ok {
		indexModel.Options.SetName(name)
		indexName = name
	}
	if unique, ok := indexDataMap["unique"].(bool); ok {
		indexModel.Options.SetUnique(unique)
	}
	if sparse, ok := indexDataMap["sparse"].(bool); ok {
		indexModel.Options.SetSparse(sparse)
	}
	if expireAfterSeconds, ok := indexDataMap["expireAfterSeconds"].(int32); ok {
		indexModel.Options.SetExpireAfterSeconds(expireAfterSeconds)
	}
	if partialFilterExpression, ok := indexDataMap["partialFilterExpression"]; ok {
		indexModel.Options.SetPartialFilterExpression(partialFilterExpression)
	}
	if collation, ok := indexDataMap["collation"]; ok {
		// Collation requires a specific struct, so we need to marshal and unmarshal
		collationBytes, err := bson.Marshal(collation)
		if err == nil {
			var collationStruct options.Collation
			err = bson.Unmarshal(collationBytes, &collationStruct)
			if err == nil {
				indexModel.Options.SetCollation(&collationStruct)
			}
		}
	}

	_, err := db.Collection(collName).Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		log.Printf("WARN: Failed to create index '%s' on collection '%s': %v", indexName, collName, err)
	} else {
		fmt.Printf("    - Created index '%s' on target collection '%s'\n", indexName, collName)
	}
}

func getIndexMap(ctx context.Context, db *mongo.Database, collName string) map[string]bson.D {
	indexMap := make(map[string]bson.D)
	cursor, err := db.Collection(collName).Indexes().List(ctx)
	if err != nil {
		log.Printf("WARN: Cannot get indexes for collection '%s' in DB '%s': %v", collName, db.Name(), err)
		return indexMap
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var index bson.D
		if err := cursor.Decode(&index); err != nil {
			log.Printf("WARN: Failed to decode index for collection %s: %v", collName, err)
			continue
		}
		indexName := index.Map()["name"].(string)
		indexMap[indexName] = index
	}
	return indexMap
}

func compareIndexes(source, target bson.D) []string {
	var reasons []string

	sourceMap := source.Map()
	targetMap := target.Map()

	// 1. Compare Keys
	if !reflect.DeepEqual(sourceMap["key"], targetMap["key"]) {
		reasons = append(reasons, fmt.Sprintf("Key mismatch (Source: %v, Target: %v)", sourceMap["key"], targetMap["key"]))
	}

	// 2. Compare other properties
	compareBsonMElement(sourceMap, targetMap, "unique", &reasons)
	compareBsonMElement(sourceMap, targetMap, "sparse", &reasons)
	compareBsonMElement(sourceMap, targetMap, "expireAfterSeconds", &reasons)
	compareBsonMElement(sourceMap, targetMap, "partialFilterExpression", &reasons)
	compareBsonMElement(sourceMap, targetMap, "collation", &reasons)

	return reasons
}

func compareBsonMElement(source bson.M, target bson.M, key string, reasons *[]string) {
	sourceVal, sOK := source[key]
	targetVal, tOK := target[key]

	if sOK != tOK {
		*reasons = append(*reasons, fmt.Sprintf("'%s' property existence mismatch (Source: %v, Target: %v)", key, sOK, tOK))
		return
	}

	if sOK { // If both exist, compare values
		if !reflect.DeepEqual(sourceVal, targetVal) {
			// For better readability, marshal maps/slices to JSON string
			sValStr, _ := json.Marshal(sourceVal)
			tValStr, _ := json.Marshal(targetVal)
			*reasons = append(*reasons, fmt.Sprintf("'%s' property value mismatch (Source: %s, Target: %s)", key, sValStr, tValStr))
		}
	}
}

func generateCreateIndexStatement(collName string, indexData bson.D) string {
	indexDataMap := indexData.Map()
	keys, ok := indexDataMap["key"].(bson.D)
	if !ok {
		return "// Error: Could not determine index keys"
	}

	var keyParts []string
	for _, elem := range keys {
		keyParts = append(keyParts, fmt.Sprintf("%s: %v", elem.Key, elem.Value))
	}
	keyString := fmt.Sprintf("{ %s }", strings.Join(keyParts, ", "))

	var optionsParts []string
	if name, ok := indexDataMap["name"].(string); ok {
		optionsParts = append(optionsParts, fmt.Sprintf("name: \"%s\"", name))
	}
	if unique, ok := indexDataMap["unique"].(bool); ok && unique {
		optionsParts = append(optionsParts, "unique: true")
	}
	if sparse, ok := indexDataMap["sparse"].(bool); ok && sparse {
		optionsParts = append(optionsParts, "sparse: true")
	}
	if expireAfterSeconds, ok := indexDataMap["expireAfterSeconds"].(int32); ok {
		optionsParts = append(optionsParts, fmt.Sprintf("expireAfterSeconds: %d", expireAfterSeconds))
	}
	if partialFilterExpression, ok := indexDataMap["partialFilterExpression"]; ok {
		jsonBytes, err := bson.MarshalExtJSON(partialFilterExpression, true, false)
		if err == nil {
			optionsParts = append(optionsParts, fmt.Sprintf("partialFilterExpression: %s", string(jsonBytes)))
		}
	}
	if collation, ok := indexDataMap["collation"]; ok {
		jsonBytes, err := bson.MarshalExtJSON(collation, true, false)
		if err == nil {
			optionsParts = append(optionsParts, fmt.Sprintf("collation: %s", string(jsonBytes)))
		}
	}

	optionsString := ""
	if len(optionsParts) > 0 {
		optionsString = fmt.Sprintf(", { %s }", strings.Join(optionsParts, ", "))
	}

	return fmt.Sprintf("db.%s.createIndex(%s%s)", collName, keyString, optionsString)
}
