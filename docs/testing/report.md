# 测试报告

## 2026-03-19: UniSearch 语义搜索接口

### 执行结果

```
go test ./baidupan/... -v -cover -count=1

=== RUN   TestFile_UniSearch
--- PASS: TestFile_UniSearch (0.00s)
=== RUN   TestFile_UniSearch_MinimalParams
--- PASS: TestFile_UniSearch_MinimalParams (0.00s)
=== RUN   TestFile_UniSearch_NilParams
--- PASS: TestFile_UniSearch_NilParams (0.00s)
=== RUN   TestFile_UniSearch_EmptyQuery
--- PASS: TestFile_UniSearch_EmptyQuery (0.00s)
=== RUN   TestFile_UniSearch_APIError
--- PASS: TestFile_UniSearch_APIError (0.00s)
=== RUN   TestFile_UniSearch_WithAllParams
--- PASS: TestFile_UniSearch_WithAllParams (0.00s)
=== RUN   TestFile_UniSearch_EmptyResult
--- PASS: TestFile_UniSearch_EmptyResult (0.00s)
=== RUN   TestFile_UniSearch_WithOCR
--- PASS: TestFile_UniSearch_WithOCR (0.00s)
=== RUN   TestScene_Search
--- PASS: TestScene_Search (0.00s)
=== RUN   TestScene_Search_MinimalParams
--- PASS: TestScene_Search_MinimalParams (0.00s)
=== RUN   TestScene_Search_NilParams
--- PASS: TestScene_Search_NilParams (0.00s)
=== RUN   TestScene_Search_EmptyQuery
--- PASS: TestScene_Search_EmptyQuery (0.00s)
=== RUN   TestScene_Search_APIError
--- PASS: TestScene_Search_APIError (0.00s)
=== RUN   TestScene_Search_NilItemInResponse
--- PASS: TestScene_Search_NilItemInResponse (0.00s)
=== RUN   TestScene_Search_WithNum
--- PASS: TestScene_Search_WithNum (0.00s)
=== RUN   TestClient_Do_UniSearchError
--- PASS: TestClient_Do_UniSearchError (0.00s)

PASS
coverage: api 92.5%, scene 94.7%
```

### 覆盖率统计

| 包 | 覆盖率 | 状态 |
|----|--------|------|
| baidupan/api | 92.5% | 达标 |
| baidupan/scene | 94.7% | 达标 |

### 新增测试用例

#### API 层 (file_test.go)

| 用例 | 说明 |
|------|------|
| TestFile_UniSearch | 正常请求，验证 POST /xpan/unisearch、JSON 请求体、响应解析 |
| TestFile_UniSearch_MinimalParams | 最小参数（仅 Query） |
| TestFile_UniSearch_NilParams | nil 参数返回错误 |
| TestFile_UniSearch_EmptyQuery | 空 Query 返回错误 |
| TestFile_UniSearch_APIError | API 错误响应处理（error_no/error_msg 格式） |
| TestFile_UniSearch_WithAllParams | 所有可选参数组合 |
| TestFile_UniSearch_EmptyResult | 空结果集处理 |
| TestFile_UniSearch_WithOCR | OCR 字段解析 |

#### Scene 层 (scene_test.go)

| 用例 | 说明 |
|------|------|
| TestScene_Search | 正常搜索，验证文件和目录结果转换 |
| TestScene_Search_MinimalParams | 最小参数调用 |
| TestScene_Search_NilParams | nil 参数错误处理 |
| TestScene_Search_EmptyQuery | 空 Query 错误处理 |
| TestScene_Search_APIError | API 错误透传 |
| TestScene_Search_NilItemInResponse | 响应中包含 nil 项的处理 |
| TestScene_Search_WithNum | Num 参数传递 |

#### Client 层 (client_test.go)

| 用例 | 说明 |
|------|------|
| TestClient_Do_UniSearchError | 第四种错误格式（error_no/error_msg）处理 |

### 边界用例覆盖

| 边界情况 | 测试位置 | 状态 |
|----------|----------|------|
| nil 参数 | TestFile_UniSearch_NilParams, TestScene_Search_NilParams | 已覆盖 |
| 空字符串 Query | TestFile_UniSearch_EmptyQuery, TestScene_Search_EmptyQuery | 已覆盖 |
| 空结果集 | TestFile_UniSearch_EmptyResult | 已覆盖 |
| API 错误响应 | TestClient_Do_UniSearchError, TestFile_UniSearch_APIError | 已覆盖 |
| 响应中包含 nil 项 | TestScene_Search_NilItemInResponse | 已覆盖 |
| 可选参数组合 | TestFile_UniSearch_WithAllParams | 已覆盖 |
| OCR 字段 | TestFile_UniSearch_WithOCR | 已覆盖 |
| Content 字段 | TestScene_Search | 已覆盖 |
