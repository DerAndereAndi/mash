# MASH Web Testing Frontend

An HTTP-based testing frontend for MASH protocol conformance testing.

## Features

- **Device Discovery**: Discover MASH devices on the local network via mDNS
- **Test Management**: Browse, filter, and run test cases
- **Test Execution**: Run tests against devices with real-time progress streaming
- **Result History**: SQLite-backed persistence of test run results
- **Web UI**: Simple web interface for managing test runs

## Installation

```bash
cd mash-go
go build ./cmd/mash-web
```

## Usage

```bash
# Start the server with default settings
./mash-web

# Start on a custom port
./mash-web -port 9000

# Specify test cases directory
./mash-web -tests /path/to/testdata/cases

# Use an in-memory database (for testing)
./mash-web -db :memory:
```

### Command-line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP server port |
| `-tests` | `./testdata/cases` | Test cases directory |
| `-db` | `./mash-web.db` | SQLite database path |
| `-log-level` | `info` | Log level: debug, info, warn, error |

## Quick Start

1. Start the server:
   ```bash
   ./mash-web -tests ./testdata/cases
   ```

2. Open the web UI:
   ```
   http://localhost:8080
   ```

3. Click "Discover Devices" to find MASH devices on your network

4. Select a device (or enter the target address manually)

5. Optionally filter tests by pattern (e.g., `TC-READ-*`)

6. Enter the device's setup code if commissioning is required

7. Click "Run Tests" to start the test run

## API Overview

See [API.md](API.md) for complete API documentation.

### Key Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Server health check |
| GET | `/api/v1/info` | Server statistics |
| GET | `/api/v1/devices` | Discover MASH devices |
| GET | `/api/v1/tests` | List test cases |
| GET | `/api/v1/tests/:id` | Get test case details |
| POST | `/api/v1/runs` | Start a test run |
| GET | `/api/v1/runs` | List test runs |
| GET | `/api/v1/runs/:id` | Get run details with results |
| GET | `/api/v1/runs/:id/stream` | SSE stream of test results |

## Examples

### Discover Devices

```bash
curl http://localhost:8080/api/v1/devices?timeout=5s
```

### List Tests

```bash
# All tests
curl http://localhost:8080/api/v1/tests

# Filter by pattern
curl "http://localhost:8080/api/v1/tests?pattern=TC-READ-*"
```

### Start a Test Run

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "target": "192.168.1.100:8443",
    "pattern": "TC-READ-*",
    "setup_code": "12345678"
  }'
```

### Stream Test Results

```bash
curl http://localhost:8080/api/v1/runs/RUN_ID/stream
```

## Architecture

```
+------------------+
|   Web Browser    |
+--------+---------+
         |
         | HTTP/JSON + Static Files
         |
+--------v---------+
|   cmd/mash-web   |
|   (HTTP Server)  |
+--------+---------+
         |
   +-----+-----+-----+
   |           |     |
   v           v     v
loader    runner  discovery
(tests)   (exec)  (devices)
   |           |
   v           v
testdata   SQLite
```

## Development

### Running Tests

```bash
go test ./cmd/mash-web/...
```

### Directory Structure

```
cmd/mash-web/
├── main.go           # Entry point
├── server.go         # HTTP server setup
├── server_test.go
├── api/
│   ├── types.go      # Request/response types
│   ├── store.go      # SQLite persistence
│   ├── store_test.go
│   ├── tests.go      # Test listing handlers
│   ├── tests_test.go
│   ├── runs.go       # Test execution handlers
│   ├── runs_test.go
│   └── devices.go    # Device discovery
├── static/
│   ├── index.html    # Web UI
│   ├── style.css
│   └── app.js
└── docs/
    ├── README.md     # This file
    └── API.md        # API documentation
```

## Integration with mash-test

The web frontend uses the same test runner infrastructure as the `mash-test` CLI:

- Test cases are loaded from the same YAML format
- The runner executes tests the same way
- Results use the same data structures

This ensures consistency between CLI and web-based testing.
