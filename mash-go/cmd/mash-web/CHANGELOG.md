# Changelog

All notable changes to MASH Web will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2024-XX-XX

### Added
- Initial release of MASH Web testing frontend
- REST API for test management
  - `GET /api/v1/health` - Health check endpoint
  - `GET /api/v1/info` - Server information
  - `GET /api/v1/tests` - List test cases (supports `?grouped=true` and `?pattern=`)
  - `GET /api/v1/tests/:id` - Get test case details
  - `GET /api/v1/testsets` - List test sets (summary)
  - `GET /api/v1/testsets/:id` - Get test set with tests
  - `POST /api/v1/tests/reload` - Reload test cases from disk
  - `GET /api/v1/tests/:id/yaml` - Get raw YAML source for a test case
  - `POST /api/v1/runs` - Start a new test run
  - `GET /api/v1/runs` - List test runs
  - `GET /api/v1/runs/:id` - Get test run details
  - `GET /api/v1/runs/:id/stream` - SSE stream for live results
  - `GET /api/v1/devices` - Discover devices via mDNS
- Web UI dashboard
  - Device discovery panel
  - Test sets browser with expand/collapse
  - Tag filter button with popover selection (AND logic)
  - Test run form with target, pattern, and setup code
  - Recent runs list
  - Dynamic reload button to refresh test cases from disk
  - View YAML button for each test (syntax highlighted modal with copy support)
- Test run modal overlay (replaces separate window)
  - Real-time test result streaming via SSE
  - Pass/fail/skip/total statistics with elapsed time
  - Progress bar
  - Scrollable log with syntax-colored status
  - Auto-scroll toggle and clear log buttons
  - Completion summary message
  - Click on run history to view past results
- SQLite persistence for test run history
- Green/orange energy-themed UI design
- Version display in footer
- Build-time version injection via ldflags

### Technical
- Embedded static files (no external dependencies)
- In-memory SQLite option for testing (`:memory:`)
- Comprehensive unit test coverage

[Unreleased]: https://github.com/mash-protocol/mash-go/compare/mash-web-v0.1.0...HEAD
[0.1.0]: https://github.com/mash-protocol/mash-go/releases/tag/mash-web-v0.1.0
