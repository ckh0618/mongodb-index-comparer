# MongoDB Index Comparer

## Overview

This program compares the indexes of two MongoDB collections and reports any discrepancies. It iterates through all collections in the target database, comparing each index with its counterpart in the source database.

## Features

- Compares indexes between two MongoDB databases.
- Provides detailed reasons for any mismatches.
- Option to hide matching indexes from the output for a cleaner report.

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
| `--hide-matching` | Hide matching indexes from the output. | `false` |

### Example

```bash
./mongodb-index-comparer --source.uri="mongodb://user:pass@source-host:27017" --source.db="production" --target.uri="mongodb://user:pass@target-host:27017" --target.db="staging" --hide-matching
```

## Output

The program outputs a detailed comparison of the indexes for each collection.

### Sample Output

```
--- Index Comparison Details ---
Source DB: production | Target DB: staging

Collection: users
  - Index: _id_                         | Match: Match
  - Index: email_1                       | Match: Mismatch (Key mismatch (Source: map[email:1], Target: map[email:-1]))
  - Index: username_1                   | Match: Mismatch (Not in Source)

Collection: products
  - Index: _id_                         | Match: Match
  - Index: sku_1                        | Match: Mismatch (Not in Target)
```
