# MASH Web Roadmap

This roadmap outlines planned features for MASH Web, targeting two primary user groups:
- **Developers**: Working on MASH protocol implementations, need quick feedback
- **Certification**: Running formal qualification tests, need documentation and traceability

The focus is on keeping things simple while providing value to both groups.

---

## v0.2.0 - Developer Experience

Focus: Fast feedback loops and debugging support.

### Planned Features

- [x] **View test YAML** - Show raw YAML source for any test case (syntax highlighted)
- [ ] **Re-run failed tests** - Quick button to retry only failed tests from a run
- [ ] **Test case details view** - Show full steps, expected behavior before running
- [ ] **Run comparison** - Side-by-side diff of two runs (what changed?)
- [ ] **Improved error display** - Expandable error details with step-by-step trace
- [ ] **Keyboard shortcuts** - Quick navigation (j/k for list, Enter to run, etc.)
- [ ] **Dark mode** - Developer-friendly theme option

---

## v0.3.0 - Reporting & Export

Focus: Documentation for certification and traceability.

### Planned Features

- [ ] **Export results** - JSON and CSV export for test runs
- [ ] **PDF report generation** - Formal test report with pass/fail summary
- [ ] **PICS file support** - Upload and view PICS, filter tests by PICS requirements
- [ ] **Run notes/annotations** - Add comments to runs for documentation
- [ ] **Trends dashboard** - Pass rate over time, flaky test detection
- [ ] **Test matrix view** - Which tests passed on which device/firmware combinations

---

## v0.4.0 - Test Management

Focus: Better organization and filtering.

### Planned Features

- [ ] **Saved filters** - Save and name frequently used test patterns
- [ ] **Test favorites** - Mark commonly run tests for quick access
- [ ] **Custom test sets** - Create ad-hoc test groups (not just by file)
- [ ] **Test dependencies** - Show which tests depend on others
- [ ] **Skip reasons tracking** - Why was a test skipped? Track PICS mismatches

---

## v0.5.0 - Execution Improvements

Focus: More control over test execution.

### Planned Features

- [ ] **Run queue** - Queue multiple runs, execute sequentially
- [ ] **Parallel test execution** - Run independent tests concurrently
- [ ] **Timeout configuration** - Per-run timeout overrides
- [ ] **Stop/cancel run** - Gracefully stop a running test suite
- [ ] **Scheduled runs** - Run tests at specific times (cron-style)

---

## Future Considerations

These are ideas that may be added later based on demand:

### Infrastructure
- [ ] Docker image for easy deployment
- [ ] CI/CD integration (GitHub Actions, GitLab CI)
- [ ] Webhook notifications on run completion

### Multi-User (if needed)
- [ ] Basic authentication (single shared password or API key)
- [ ] Run ownership (who started which run)
- [ ] Shared vs personal test configurations

### Advanced
- [ ] Device profiles (store target configurations)
- [ ] Firmware version tracking
- [ ] Regression alerts (email/Slack when tests start failing)
- [ ] Test coverage visualization

---

## Non-Goals

To keep MASH Web simple, we explicitly avoid:

- Complex user management / RBAC
- Cloud-hosted SaaS version
- Real-time collaboration features
- Test case editing in the UI (use YAML files directly)
- Integration with external test management tools (Jira, TestRail)

---

## Feedback

Have a feature request? Open an issue at:
https://github.com/mash-protocol/mash-go/issues

Label it with `mash-web` and describe:
1. What problem you're trying to solve
2. Whether you're a developer or doing certification work
3. How important it is for your workflow
