# MongoDB Index Comparer

소스 DB와 타깃 DB의 **컬렉션별 문서 수**와 **인덱스 정의**를 비교하고, 필요 시 타깃 인덱스를 정리/재생성할 수 있는 Go CLI 도구입니다.

## 주요 기능

- 소스 DB의 컬렉션 목록을 기준으로 순회 비교
- 컬렉션별 문서 수 비교 (`CountDocuments`)
- 인덱스 비교
  - key
  - unique
  - sparse
  - expireAfterSeconds(TTL)
  - partialFilterExpression
  - collation
- 불일치 사유를 상세 문자열로 출력
- `--hide-matching` 옵션으로 일치 항목 숨김
- `--compare-only-index` 옵션으로 문서 수 비교 스킵
- `--force-create-index` 옵션으로 타깃 인덱스 자동 정리/생성
  - 타깃에만 존재하는 인덱스는 삭제
  - 양쪽에 동일 이름 인덱스가 있으나 속성이 다르면 타깃 인덱스 삭제 후 소스 기준으로 재생성
  - 소스에만 존재하는 인덱스는 타깃에 생성

## 동작 방식 요약

1. 소스/타깃 MongoDB에 연결 및 Ping
2. 소스 DB의 컬렉션 목록 조회
3. 각 컬렉션에 대해:
   - (기본) 소스/타깃 문서 수 비교
   - 소스/타깃 인덱스 맵 구성 후 이름 기준 비교
4. 결과 출력 및 (옵션) 타깃 인덱스 변경

> 참고: 컬렉션 비교 기준은 **소스 DB에 존재하는 컬렉션**입니다.

## 요구 사항

- Go 1.24.5+
- 소스/타깃 MongoDB 접근 권한

## 빌드

```bash
go build -o mongodb-index-comparer .
```

## 실행

```bash
./mongodb-index-comparer [flags]
```

## 옵션

| Flag | 설명 | 기본값 |
|---|---|---|
| `--source.uri` | 소스 MongoDB URI | `mongodb://localhost:27017` |
| `--target.uri` | 타깃 MongoDB URI | `mongodb://localhost:27017` |
| `--source.db` | 소스 DB 이름 | `source-db` |
| `--target.db` | 타깃 DB 이름 | `target-db` |
| `--source.filter` | 소스 문서 수 비교용 JSON 필터(Extended JSON) | `{}` |
| `--target.filter` | 타깃 문서 수 비교용 JSON 필터(Extended JSON) | `{}` |
| `--hide-matching` | 일치 항목 숨김 | `false` |
| `--force-create-index` | 타깃 인덱스 자동 정리/생성 수행 | `false` |
| `--compare-only-index` | 문서 수 비교 생략, 인덱스만 비교 | `false` |

## 사용 예시

### 1) 기본 비교

```bash
./mongodb-index-comparer \
  --source.uri="mongodb://user:pass@source-host:27017" \
  --source.db="production" \
  --target.uri="mongodb://user:pass@target-host:27017" \
  --target.db="staging"
```

### 2) 서로 다른 필터로 문서 수 비교

```bash
./mongodb-index-comparer \
  --source.uri="mongodb://localhost:27017" \
  --source.db="analytics" \
  --source.filter='{"status":"active"}' \
  --target.uri="mongodb://localhost:27017" \
  --target.db="analytics_archive" \
  --target.filter='{"status":"enabled"}'
```

### 3) 인덱스만 비교 + 일치 항목 숨김

```bash
./mongodb-index-comparer \
  --source.db="production" \
  --target.db="staging" \
  --compare-only-index \
  --hide-matching
```

### 4) 타깃 인덱스 자동 정리/생성

```bash
./mongodb-index-comparer \
  --source.db="production" \
  --target.db="staging" \
  --force-create-index
```

## 출력 예시

```text
Fetching collections from target database 'staging'...

--- Comparison Details ---
Source DB: production (Filter: {"status":"active"}) | Target DB: staging (Filter: {"status":"enabled"})

Collection: users
  - Document Count | Match: Mismatch (Source: 150, Target: 145)
  - Index: _id_                           | Match: Match
  - Index: email_1                        | Match: Mismatch (Key mismatch (Source: [{email 1}], Target: [{email -1}]))
  - Index: username_1                     | Match: Mismatch (Not in Target)
    - Create Index Statement: db.users.createIndex({ username: 1 }, { name: "username_1" })
```

## 주의사항

- `--force-create-index`는 실제로 타깃 인덱스를 변경합니다. 운영 환경에서는 사전 검증 후 사용하세요.
- 필터 파싱은 MongoDB Extended JSON 파서를 사용합니다. JSON 문법 오류 시 즉시 종료됩니다.
- 인덱스 비교는 인덱스 이름 + 주요 속성 기반이며, 비교 대상 외 옵션은 불일치로 잡히지 않을 수 있습니다.
