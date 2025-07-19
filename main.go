package main

import (
	"context"
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
	hideMatching := flag.Bool("hide-matching", false, "Hide matching indexes from the output")

	flag.Parse()
	// ------------------------------------

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // 컨텍스트 시간 연장
	defer cancel()

	// Source Client 연결
	fmt.Printf("Connecting to source MongoDB at %s...\n", *sourceURI)
	sourceClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*sourceURI))
	if err != nil {
		log.Fatalf("Failed to connect to source MongoDB: %v", err)
	}
	defer sourceClient.Disconnect(ctx)
	fmt.Println("Successfully connected to Source MongoDB.")

	// Target Client 연결
	fmt.Printf("Connecting to target MongoDB at %s...\n", *targetURI)
	targetClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*targetURI))
	if err != nil {
		log.Fatalf("Failed to connect to target MongoDB: %v", err)
	}
	defer targetClient.Disconnect(ctx)
	fmt.Println("Successfully connected to Target MongoDB.")

	sourceDB := sourceClient.Database(*sourceDBName)
	targetDB := targetClient.Database(*targetDBName)

	// Target DB의 모든 Collection 목록 가져오기
	fmt.Printf("Fetching collections from target database '%s'...\n", *targetDBName)
	collections, err := targetDB.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		log.Fatalf("Failed to list collections from target DB '%s': %v", *targetDBName, err)
	}

	fmt.Printf("\n--- Index Comparison Details ---\n")
	fmt.Printf("Source DB: %s | Target DB: %s\n", *sourceDBName, *targetDBName)

	// 각 Collection 순회하며 인덱스 비교
	for _, collName := range collections {
		fmt.Printf("\nCollection: %s\n", collName)

		sourceIndexMap := getIndexMap(ctx, sourceDB, collName)
		targetIndexMap := getIndexMap(ctx, targetDB, collName)

		// 인덱스 비교 및 결과 출력
		for name, targetIndex := range targetIndexMap {
			if sourceIndex, exists := sourceIndexMap[name]; exists {
				// 이름이 같은 인덱스가 존재하면 상세 비교 수행
				reasons := compareIndexes(sourceIndex, targetIndex)
                if len(reasons) == 0 {
                    if !*hideMatching {
                        fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Match")
                    }
                } else {
                    reasonStr := strings.Join(reasons, ", ")
                    fmt.Printf("  - Index: %-30s | Match: %s (%s)\n", name, "Mismatch", reasonStr)
                }
				// 비교가 끝난 인덱스는 source 맵에서 제거
				delete(sourceIndexMap, name)
			} else {
				// Source에 해당 이름의 인덱스가 없음
				fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Mismatch (Not in Source)")
			}
		}

		// Source 맵에 남아있는 인덱스들은 Target에 없는 것들
		for name := range sourceIndexMap {
			fmt.Printf("  - Index: %-30s | Match: %s\n", name, "Mismatch (Not in Target)")
		}
	}
}

// getIndexMap returns a map of index information for a given database and collection name.
func getIndexMap(ctx context.Context, db *mongo.Database, collName string) map[string]bson.M {
	indexMap := make(map[string]bson.M)
	cursor, err := db.Collection(collName).Indexes().List(ctx)
	if err != nil {
		// Log a warning and continue if an error occurs (e.g., collection not found in source)
		log.Printf("WARN: Cannot get indexes for collection '%s' in DB '%s': %v", collName, db.Name(), err)
		return indexMap
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			log.Printf("WARN: Failed to decode index for collection %s: %v", collName, err)
			continue
		}
		indexName := index["name"].(string)
		indexMap[indexName] = index
	}
	return indexMap
}

// compareIndexes compares the details of two indexes and returns the differences as a string slice.
func compareIndexes(source, target bson.M) []string {
	var reasons []string

	// 1. Compare Keys
	sourceKey := source["key"]
	targetKey := target["key"]
	if !reflect.DeepEqual(sourceKey, targetKey) {
		reasons = append(reasons, fmt.Sprintf("Key mismatch (Source: %v, Target: %v)", sourceKey, targetKey))
	}

	// 2. Compare Unique property
	sourceUnique, sOK := source["unique"].(bool)
	targetUnique, tOK := target["unique"].(bool)
	if sOK != tOK || sourceUnique != targetUnique {
		reasons = append(reasons, fmt.Sprintf("Unique property mismatch (Source: %v, Target: %v)", sourceUnique, targetUnique))
	}

	// 3. Compare Sparse property
	sourceSparse, sOK := source["sparse"].(bool)
	targetSparse, tOK := target["sparse"].(bool)
	if sOK != tOK || sourceSparse != targetSparse {
		reasons = append(reasons, fmt.Sprintf("Sparse property mismatch (Source: %v, Target: %v)", sourceSparse, targetSparse))
	}
    
    // 4. Compare TTL (expireAfterSeconds) property
    sourceTTL, sOK := source["expireAfterSeconds"].(int32)
    targetTTL, tOK := target["expireAfterSeconds"].(int32)
    if sOK != tOK || sourceTTL != targetTTL {
        reasons = append(reasons, fmt.Sprintf("TTL(expireAfterSeconds) property mismatch (Source: %v, Target: %v)", sourceTTL, targetTTL))
    }

	// TODO: You can add other properties to compare as needed (e.g., collation, partialFilterExpression).

	return reasons
}
