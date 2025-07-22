# MongoDB Index and Document Count Comparer

## Overview

This program compares the indexes and document counts of collections between two MongoDB databases. It iterates through all collections in the target database, comparing each index and the document count with its counterpart in the source database. You can also provide separate filters for the source and target databases to compare specific subsets of your data.

## Features

- Compares indexes between two MongoDB databases.
- Compares document counts, with optional filters for both source and target.
- Provides detailed reasons for any mismatches in indexes.
- Option to hide matching indexes and counts from the output for a cleaner report.

## Usage

### Prerequisites

- Go 1.18 or higher.
- Access to the source and target MongoDB instances.

### Building the Program

```bash
go build
```

### Running the Program

```bash
./mongodb-index-comparer [flags]
```

### Flags

| Flag | Description | Default |
|---|---|---|
| `--source.uri` | Source MongoDB connection URI. | `mongodb://localhost:27017` |
| `--target.uri` | Target MongoDB connection URI. | `mongodb://localhost:27017` |
| `--source.db` | Source database name. | `source-db` |
| `--target.db` | Target database name. | `target-db` |
| `--source.filter` | Source collection filter as a JSON string. | `{}` |
| `--target.filter` | Target collection filter as a JSON string. | `{}` |
| `--hide-matching` | Hide matching indexes and counts from the output. | `false` |

### Example

#### Basic Comparison
```bash
./mongodb-index-comparer --source.uri="mongodb://user:pass@source-host:27017" --source.db="production" --target.uri="mongodb://user:pass@target-host:27017" --target.db="staging" --hide-matching
```

#### Comparison with Filters
This example compares documents where the `status` field is "active" in the source and "enabled" in the target.
```bash
./mongodb-index-comparer \
  --source.uri="mongodb://localhost:27017" \
  --source.db="analytics" \
  --source.filter='{"status": "active"}' \
  --target.uri="mongodb://localhost:27017" \
  --target.db="analytics_archive" \
  --target.filter='{"status": "enabled"}'
```

## Output

The program outputs a detailed comparison for each collection.

### Sample Output

```
--- Comparison Details ---
Source DB: production (Filter: {"status":"active"}) | Target DB: staging (Filter: {"status":"enabled"})

Collection: users
  - Document Count | Match: Mismatch (Source: 150, Target: 145)
  - Index: _id_                         | Match: Match
  - Index: email_1                       | Match: Mismatch (Key mismatch (Source: map[email:1], Target: map[email:-1]))
  - Index: username_1                   | Match: Mismatch (Not in Source)

Collection: products
  - Document Count | Match: Match (Source: 5000, Target: 5000)
  - Index: _id_                         | Match: Match
  - Index: sku_1                        | Match: Mismatch (Not in Target)
```

```
