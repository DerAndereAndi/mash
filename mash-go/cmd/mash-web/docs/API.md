# MASH Web API Documentation

Base URL: `http://localhost:8080/api/v1`

## Health & Info

### GET /health

Returns server health status.

**Response:**
```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

### GET /info

Returns server statistics.

**Response:**
```json
{
  "test_count": 506,
  "run_count": 42
}
```

## Device Discovery

### GET /devices

Discovers MASH devices on the local network via mDNS.

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `timeout` | string | `10s` | Discovery timeout (e.g., "5s", "30s") |

**Response:**
```json
{
  "devices": [
    {
      "instance_name": "MASH-1234",
      "host": "evse-001.local",
      "port": 8443,
      "addresses": ["192.168.1.100"],
      "discriminator": 1234,
      "brand": "ACME",
      "model": "EVSE-Pro",
      "device_name": "Garage Charger",
      "serial": "SN123456"
    }
  ],
  "discovered_at": "2024-01-15T10:30:00Z",
  "timeout": "10s"
}
```

## Test Cases

### GET /tests

Lists all test cases with optional filtering.

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `pattern` | string | Filter tests by ID or name pattern (e.g., "TC-READ-*") |

**Response:**
```json
{
  "tests": [
    {
      "id": "TC-READ-001",
      "name": "Basic read test",
      "description": "Tests basic read functionality",
      "pics_requirements": ["MASH.S.TRANS"],
      "tags": ["read", "basic"],
      "step_count": 5,
      "timeout": "30s",
      "skip": false,
      "skip_reason": ""
    }
  ],
  "total": 506
}
```

### GET /tests?grouped=true

Lists all test cases grouped by test set.

**Response:**
```json
{
  "sets": [
    {
      "id": "read-tests",
      "name": "Read Tests",
      "description": "Tests for read operations",
      "file_name": "read-tests.yaml",
      "test_count": 15,
      "tags": ["read", "basic"],
      "tests": [
        {
          "id": "TC-READ-001",
          "name": "Basic read test",
          "step_count": 5
        }
      ]
    }
  ],
  "total": 506
}
```

### POST /tests/reload

Reloads test cases from disk. Use this when test YAML files have been modified.

**Response:**
```json
{
  "status": "reloaded",
  "tests": 506,
  "test_sets": 25
}
```

### GET /testsets

Lists all test sets (summary only, without individual tests).

**Response:**
```json
{
  "sets": [
    {
      "id": "read-tests",
      "name": "Read Tests",
      "description": "Tests for read operations",
      "file_name": "read-tests.yaml",
      "test_count": 15,
      "tags": ["read", "basic"]
    }
  ],
  "total": 25
}
```

### GET /testsets/:id

Gets a specific test set with all its tests.

**Path Parameters:**
| Parameter | Description |
|-----------|-------------|
| `id` | Test set ID (derived from filename) |

**Response:**
```json
{
  "id": "read-tests",
  "name": "Read Tests",
  "description": "Tests for read operations",
  "file_name": "read-tests.yaml",
  "test_count": 15,
  "tags": ["read", "basic"],
  "tests": [
    {
      "id": "TC-READ-001",
      "name": "Basic read test",
      "step_count": 5,
      "tags": ["read", "basic"]
    }
  ]
}
```

### GET /tests/:id

Gets details for a specific test case.

### GET /tests/:id/yaml

Gets the raw YAML source for a specific test case.

**Path Parameters:**
| Parameter | Description |
|-----------|-------------|
| `id` | Test case ID (e.g., "TC-READ-001") |

**Response:**
```json
{
  "test_id": "TC-READ-001",
  "filename": "read-tests.yaml",
  "yaml": "id: TC-READ-001\nname: Basic read test\n..."
}
```

**Path Parameters:**
| Parameter | Description |
|-----------|-------------|
| `id` | Test case ID (e.g., "TC-READ-001") |

**Response:**
```json
{
  "id": "TC-READ-001",
  "name": "Basic read test",
  "description": "Tests basic read functionality",
  "pics_requirements": ["MASH.S.TRANS"],
  "tags": ["read", "basic"],
  "step_count": 5,
  "timeout": "30s"
}
```

**Error Response (404):**
```json
{
  "error": "Test case not found",
  "details": "TC-INVALID-ID"
}
```

## Test Runs

### POST /runs

Starts a new test run.

**Request Body:**
```json
{
  "target": "192.168.1.100:8443",
  "pattern": "TC-READ-*",
  "setup_code": "12345678",
  "mode": "device",
  "timeout": "30s"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `target` | string | Yes | Target device address (host:port) |
| `pattern` | string | No | Test filter pattern |
| `setup_code` | string | No | PASE setup code for commissioning |
| `mode` | string | No | Test mode: "device" or "controller" (default: "device") |
| `timeout` | string | No | Test timeout (default: "30s") |

**Response (202 Accepted):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "target": "192.168.1.100:8443",
  "pattern": "TC-READ-*",
  "status": "running",
  "started_at": "2024-01-15T10:30:00Z"
}
```

### GET /runs

Lists all test runs.

**Response:**
```json
{
  "runs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "target": "192.168.1.100:8443",
      "pattern": "TC-READ-*",
      "status": "completed",
      "started_at": "2024-01-15T10:30:00Z",
      "completed_at": "2024-01-15T10:35:00Z",
      "pass_count": 45,
      "fail_count": 2,
      "skip_count": 5,
      "total_count": 52,
      "duration": "5m0s"
    }
  ],
  "total": 42
}
```

**Run Status Values:**
| Status | Description |
|--------|-------------|
| `pending` | Run created but not started |
| `running` | Tests currently executing |
| `completed` | All tests finished successfully |
| `failed` | Run failed due to error |

### GET /runs/:id

Gets details for a specific run including test results.

**Path Parameters:**
| Parameter | Description |
|-----------|-------------|
| `id` | Run ID (UUID) |

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "target": "192.168.1.100:8443",
  "pattern": "TC-READ-*",
  "status": "completed",
  "started_at": "2024-01-15T10:30:00Z",
  "completed_at": "2024-01-15T10:35:00Z",
  "pass_count": 45,
  "fail_count": 2,
  "skip_count": 5,
  "total_count": 52,
  "duration": "5m0s",
  "results": [
    {
      "test_id": "TC-READ-001",
      "test_name": "Basic read test",
      "status": "passed",
      "duration": "150ms",
      "steps": [
        {
          "index": 0,
          "action": "connect",
          "status": "passed",
          "duration": "50ms"
        },
        {
          "index": 1,
          "action": "read",
          "status": "passed",
          "duration": "100ms",
          "expects": {
            "read_success": {
              "passed": true,
              "expected": true,
              "actual": true,
              "message": "read succeeded"
            }
          }
        }
      ]
    },
    {
      "test_id": "TC-READ-002",
      "test_name": "Read with filter",
      "status": "failed",
      "duration": "200ms",
      "error": "assertion failed: expected 'ok', got 'error'"
    },
    {
      "test_id": "TC-READ-003",
      "test_name": "Filtered read",
      "status": "skipped",
      "skip_reason": "PICS requirement not met: MASH.S.FILTER"
    }
  ]
}
```

**Test Result Status Values:**
| Status | Description |
|--------|-------------|
| `passed` | Test completed successfully |
| `failed` | Test failed (see error field) |
| `skipped` | Test skipped (see skip_reason field) |

### GET /runs/:id/stream

Server-Sent Events stream of test results for a running test.

**Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Events:**

`result` - Emitted for each completed test:
```
event: result
data: {"test_id":"TC-READ-001","test_name":"Basic read test","status":"passed","duration":"150ms"}

event: result
data: {"test_id":"TC-READ-002","test_name":"Read with filter","status":"failed","error":"assertion failed"}
```

`done` - Emitted when the run completes:
```
event: done
data: {}
```

**Example Client (JavaScript):**
```javascript
const source = new EventSource('/api/v1/runs/RUN_ID/stream');

source.addEventListener('result', (event) => {
  const result = JSON.parse(event.data);
  console.log(`[${result.status}] ${result.test_id}: ${result.test_name}`);
});

source.addEventListener('done', () => {
  source.close();
  console.log('Run completed');
});
```

## Error Responses

All endpoints may return error responses in the following format:

```json
{
  "error": "Error message",
  "details": "Additional details (optional)"
}
```

**Common HTTP Status Codes:**
| Code | Description |
|------|-------------|
| 200 | Success |
| 202 | Accepted (for async operations) |
| 400 | Bad Request (invalid input) |
| 404 | Not Found |
| 405 | Method Not Allowed |
| 500 | Internal Server Error |

## Rate Limiting

Currently no rate limiting is implemented. For production use, consider adding a reverse proxy with rate limiting.

## CORS

The API allows cross-origin requests from any origin. For production, configure appropriate CORS restrictions.
