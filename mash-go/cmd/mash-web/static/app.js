// MASH Test Runner - Frontend Application

const API_BASE = '/api/v1';

// State
let testSets = [];
let devices = [];
let runs = [];
let expandedSets = new Set();
let activeTagFilters = new Set();
let allTags = [];

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    checkHealth();
    loadTestSets();
    loadRuns();
});

// Health check
async function checkHealth() {
    try {
        const response = await fetch(`${API_BASE}/health`);
        const data = await response.json();

        const indicator = document.getElementById('server-status');
        if (data.status === 'ok') {
            indicator.classList.remove('error');
            indicator.title = `Server OK (v${data.version})`;
            // Show version in footer
            const versionEl = document.getElementById('app-version');
            if (versionEl) {
                versionEl.textContent = `v${data.version}`;
            }
        } else {
            indicator.classList.add('error');
            indicator.title = 'Server error';
        }
    } catch (error) {
        document.getElementById('server-status').classList.add('error');
        console.error('Health check failed:', error);
    }
}

// Device discovery
async function discoverDevices() {
    const btn = document.getElementById('discover-btn');
    const status = document.getElementById('discover-status');

    btn.disabled = true;
    status.innerHTML = '<span class="spinner"></span> Discovering...';

    try {
        const response = await fetch(`${API_BASE}/devices?timeout=5s`);
        const data = await response.json();

        devices = data.devices || [];
        renderDevices();
        status.textContent = `Found ${devices.length} device(s)`;
    } catch (error) {
        status.textContent = `Error: ${error.message}`;
        console.error('Discovery failed:', error);
    } finally {
        btn.disabled = false;
    }
}

function renderDevices() {
    const container = document.getElementById('devices-list');

    if (devices.length === 0) {
        container.innerHTML = '<div class="empty-state">No devices found. Click "Discover Devices" to scan.</div>';
        return;
    }

    container.innerHTML = devices.map(device => `
        <div class="list-item device-item" onclick="selectDevice('${device.addresses[0] || device.host}', ${device.port})">
            <div>
                <div class="device-name">${device.device_name || device.instance_name}</div>
                <div class="device-info">${device.brand || ''} ${device.model || ''}</div>
            </div>
            <div class="device-info">
                ${device.addresses[0] || device.host}:${device.port}
            </div>
        </div>
    `).join('');
}

function selectDevice(host, port) {
    document.getElementById('target').value = `${host}:${port}`;
}

// Reload tests from disk
async function reloadTests() {
    const btn = document.getElementById('reload-btn');
    btn.disabled = true;
    btn.textContent = 'Reloading...';

    try {
        const response = await fetch(`${API_BASE}/tests/reload`, { method: 'POST' });
        const data = await response.json();

        if (response.ok) {
            // Clear caches and reload
            testSets = [];
            allTags = [];
            activeTagFilters.clear();
            expandedSets.clear();
            await loadTestSets();
            btn.textContent = 'Reloaded!';
            setTimeout(() => { btn.textContent = 'Reload'; }, 1500);
        } else {
            throw new Error(data.error || 'Failed to reload');
        }
    } catch (error) {
        console.error('Reload failed:', error);
        btn.textContent = 'Error';
        setTimeout(() => { btn.textContent = 'Reload'; }, 2000);
    } finally {
        btn.disabled = false;
    }
}

// Test sets
async function loadTestSets() {
    try {
        const response = await fetch(`${API_BASE}/tests?grouped=true`);
        const data = await response.json();

        testSets = data.sets || [];

        // Collect all unique tags
        const tagSet = new Set();
        testSets.forEach(set => {
            set.tests.forEach(test => {
                (test.tags || []).forEach(tag => tagSet.add(tag));
            });
        });
        allTags = Array.from(tagSet).sort();

        renderTagFilter();
        renderTestSets();
        document.getElementById('test-count').textContent = `${data.total} tests in ${testSets.length} sets`;
    } catch (error) {
        console.error('Failed to load test sets:', error);
    }
}

// Tag counts cache
let tagCounts = {};

function renderTagFilter() {
    // Count tests per tag
    tagCounts = {};
    testSets.forEach(set => {
        set.tests.forEach(test => {
            (test.tags || []).forEach(tag => {
                tagCounts[tag] = (tagCounts[tag] || 0) + 1;
            });
        });
    });

    // Update button label
    const btn = document.getElementById('tag-filter-btn');
    const label = document.getElementById('tag-filter-label');
    if (activeTagFilters.size > 0) {
        label.textContent = `Tags (${activeTagFilters.size})`;
        btn.classList.add('active');
    } else {
        label.textContent = 'Filter by tag';
        btn.classList.remove('active');
    }

    // Render active tags
    const activeContainer = document.getElementById('active-tags');
    if (activeTagFilters.size > 0) {
        activeContainer.innerHTML = Array.from(activeTagFilters).map(tag => `
            <span class="active-tag">
                ${tag}
                <span class="active-tag-remove" onclick="event.stopPropagation(); removeTagFilter('${tag}')">&times;</span>
            </span>
        `).join('');
    } else {
        activeContainer.innerHTML = '';
    }

    // Render popover list
    const popoverList = document.getElementById('tag-popover-list');
    if (allTags.length === 0) {
        popoverList.innerHTML = '<span style="color: var(--text-muted); font-size: 0.75rem;">No tags available</span>';
    } else {
        popoverList.innerHTML = allTags.map(tag => `
            <span class="tag-popover-item ${activeTagFilters.has(tag) ? 'selected' : ''}"
                  onclick="toggleTagFilter('${tag}')">
                ${tag}
                <span class="tag-popover-count">${tagCounts[tag]}</span>
            </span>
        `).join('');
    }
}

function toggleTagPopover(event) {
    if (event) event.stopPropagation();

    const popover = document.getElementById('tag-popover');

    if (!popover) {
        console.error('Popover element not found!');
        return;
    }

    const isActive = popover.classList.contains('active');
    console.log('toggleTagPopover:', isActive ? 'closing' : 'opening');

    if (isActive) {
        popover.classList.remove('active');
        popover.style.display = 'none';
        document.removeEventListener('click', closeTagPopoverOnClickOutside);
    } else {
        popover.classList.add('active');
        popover.style.display = 'flex';
        // Close when clicking outside (delayed to avoid immediate trigger)
        setTimeout(() => {
            document.addEventListener('click', closeTagPopoverOnClickOutside);
        }, 10);
    }
}

function closeTagPopover() {
    const popover = document.getElementById('tag-popover');
    if (popover) {
        popover.classList.remove('active');
        popover.style.display = 'none';
    }
    document.removeEventListener('click', closeTagPopoverOnClickOutside);
}

function closeTagPopoverOnClickOutside(e) {
    const container = document.querySelector('.tag-filter-container');

    if (!container.contains(e.target)) {
        closeTagPopover();
    }
}

function toggleTagFilter(tag) {
    if (activeTagFilters.has(tag)) {
        activeTagFilters.delete(tag);
    } else {
        activeTagFilters.add(tag);
    }
    renderTagFilter();
    renderTestSets();
}

function removeTagFilter(tag) {
    activeTagFilters.delete(tag);
    renderTagFilter();
    renderTestSets();
}

function clearTagFilters() {
    activeTagFilters.clear();
    renderTagFilter();
    renderTestSets();
    // Close popover
    document.getElementById('tag-popover').classList.remove('active');
}

function filterTests() {
    renderTestSets();
}

function testMatchesTags(test) {
    if (activeTagFilters.size === 0) return true;
    const testTags = new Set(test.tags || []);
    // Test must have ALL selected tags (AND logic)
    for (const tag of activeTagFilters) {
        if (!testTags.has(tag)) return false;
    }
    return true;
}

function renderTestSets() {
    const container = document.getElementById('tests-list');
    const filter = document.getElementById('test-filter').value.toLowerCase();

    // Filter sets and tests by text and tags
    const filtered = testSets.map(set => {
        const filteredTests = set.tests.filter(test => {
            // Text filter
            const matchesText = !filter ||
                test.id.toLowerCase().includes(filter) ||
                test.name.toLowerCase().includes(filter);
            // Tag filter
            const matchesTags = testMatchesTags(test);
            return matchesText && matchesTags;
        });
        return { ...set, tests: filteredTests, test_count: filteredTests.length };
    }).filter(set => set.test_count > 0);

    const totalTests = filtered.reduce((sum, set) => sum + set.test_count, 0);
    const filterInfo = activeTagFilters.size > 0
        ? ` (filtered by: ${Array.from(activeTagFilters).join(', ')})`
        : '';
    document.getElementById('test-count').textContent = `${totalTests} tests in ${filtered.length} sets${filterInfo}`;

    if (filtered.length === 0) {
        container.innerHTML = '<div class="empty-state">No matching tests found.</div>';
        return;
    }

    container.innerHTML = filtered.map(set => `
        <div class="test-set" data-set-id="${set.id}">
            <div class="test-set-header" onclick="toggleSet('${set.id}')">
                <div class="test-set-expand">${expandedSets.has(set.id) ? '▼' : '▶'}</div>
                <div class="test-set-info">
                    <div class="test-set-name">${set.name}</div>
                    <div class="test-set-meta">
                        <span class="test-set-count">${set.test_count} tests</span>
                        <span class="test-set-file">${set.file_name}</span>
                    </div>
                    ${set.description ? `<div class="test-set-desc">${set.description}</div>` : ''}
                </div>
                <div class="test-set-actions">
                    ${set.tags?.length ? `
                        <div class="test-set-tags">
                            ${set.tags.slice(0, 3).map(tag => `<span class="tag tag-clickable ${activeTagFilters.has(tag) ? 'active' : ''}" onclick="event.stopPropagation(); toggleTagFilter('${tag}')">${tag}</span>`).join('')}
                            ${set.tags.length > 3 ? `<span class="tag tag-more">+${set.tags.length - 3}</span>` : ''}
                        </div>
                    ` : ''}
                    <button class="btn-small" onclick="event.stopPropagation(); runSet('${set.id}')">Run Set</button>
                </div>
            </div>
            <div class="test-set-tests ${expandedSets.has(set.id) ? '' : 'collapsed'}">
                ${set.tests.map(test => `
                    <div class="test-item" onclick="event.stopPropagation(); selectTest('${test.id}')">
                        <div class="test-main">
                            <span class="test-id">${test.id}</span>
                            <span class="test-name">${test.name}</span>
                            <button class="btn-yaml" onclick="event.stopPropagation(); showYaml('${test.id}')" title="View YAML source">YAML</button>
                        </div>
                        <div class="test-meta">
                            <span class="test-steps">${test.step_count} steps</span>
                            ${test.timeout ? `<span class="test-timeout">${test.timeout}</span>` : ''}
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
    `).join('');
}

function toggleSet(setId) {
    if (expandedSets.has(setId)) {
        expandedSets.delete(setId);
    } else {
        expandedSets.add(setId);
    }
    renderTestSets();
}

function selectTest(testId) {
    document.getElementById('pattern').value = testId;
}

function runSet(setId) {
    // Find the set and create a pattern that matches all its test IDs
    const set = testSets.find(s => s.id === setId);
    if (set && set.tests.length > 0) {
        // Use the common prefix of test IDs if available, otherwise list them
        const ids = set.tests.map(t => t.id);
        const commonPrefix = findCommonPrefix(ids);
        if (commonPrefix.length > 3) {
            document.getElementById('pattern').value = commonPrefix + '*';
        } else {
            // Use set file name as pattern hint
            document.getElementById('pattern').value = ids[0].split('-').slice(0, 2).join('-') + '-*';
        }
    }
}

function findCommonPrefix(strings) {
    if (strings.length === 0) return '';
    let prefix = strings[0];
    for (let i = 1; i < strings.length; i++) {
        while (strings[i].indexOf(prefix) !== 0) {
            prefix = prefix.substring(0, prefix.length - 1);
            if (prefix === '') return '';
        }
    }
    return prefix;
}

// Test runs
async function loadRuns() {
    try {
        const response = await fetch(`${API_BASE}/runs`);
        const data = await response.json();

        runs = data.runs || [];
        renderRuns();
    } catch (error) {
        console.error('Failed to load runs:', error);
    }
}

function renderRuns() {
    const container = document.getElementById('runs-list');

    if (runs.length === 0) {
        container.innerHTML = '<div class="empty-state">No test runs yet.</div>';
        return;
    }

    container.innerHTML = runs.slice(0, 20).map(run => `
        <div class="list-item run-item" onclick="viewRun('${run.id}')">
            <div>
                <div class="run-target">${run.target}</div>
                <div class="device-info">${run.pattern || 'All tests'}</div>
            </div>
            <div class="run-stats">
                <span class="run-status ${run.status}">${run.status}</span>
                ${run.status === 'completed' || run.status === 'failed' ? `
                    <span class="status-passed">${run.pass_count}P</span>
                    <span class="status-failed">${run.fail_count}F</span>
                    <span class="status-skipped">${run.skip_count}S</span>
                ` : ''}
            </div>
        </div>
    `).join('');
}

async function startRun(event) {
    event.preventDefault();

    const target = document.getElementById('target').value;
    const pattern = document.getElementById('pattern').value;
    const setupCode = document.getElementById('setup-code').value;

    const btn = document.getElementById('run-btn');
    btn.disabled = true;
    btn.textContent = 'Starting...';

    try {
        const response = await fetch(`${API_BASE}/runs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                target,
                pattern: pattern || undefined,
                setup_code: setupCode || undefined
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to start run');
        }

        const run = await response.json();

        // Show run in modal
        showRunModal(run.id, target, pattern);

        // Refresh runs list
        loadRuns();

    } catch (error) {
        alert(`Error: ${error.message}`);
    } finally {
        btn.disabled = false;
        btn.textContent = 'Run Tests';
    }
}

function viewRun(runId) {
    // Show run in modal
    showRunModalFromHistory(runId);
}

// YAML Viewer Modal
let currentYaml = '';

async function showYaml(testId) {
    const modal = document.getElementById('yaml-modal');
    const content = document.getElementById('yaml-content');
    const title = document.getElementById('yaml-modal-title');
    const copyStatus = document.getElementById('copy-status');

    title.textContent = testId;
    content.innerHTML = '<span style="color: var(--text-muted)">Loading...</span>';
    copyStatus.textContent = '';
    modal.classList.add('active');

    try {
        const response = await fetch(`${API_BASE}/tests/${testId}/yaml`);
        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Failed to load YAML');
        }

        currentYaml = data.yaml;
        content.innerHTML = highlightYaml(data.yaml);
    } catch (error) {
        content.innerHTML = `<span style="color: var(--error-color)">Error: ${error.message}</span>`;
        currentYaml = '';
    }
}

function closeYamlModal(event) {
    if (event && event.target !== event.currentTarget) return;
    document.getElementById('yaml-modal').classList.remove('active');
    currentYaml = '';
}

async function copyYaml() {
    if (!currentYaml) return;

    try {
        await navigator.clipboard.writeText(currentYaml);
        document.getElementById('copy-status').textContent = 'Copied!';
        setTimeout(() => {
            document.getElementById('copy-status').textContent = '';
        }, 2000);
    } catch (error) {
        document.getElementById('copy-status').textContent = 'Failed to copy';
    }
}

function highlightYaml(yaml) {
    // Simple YAML syntax highlighting
    return yaml
        .split('\n')
        .map(line => {
            // Comments
            if (line.trim().startsWith('#')) {
                return `<span class="yaml-comment">${escapeHtml(line)}</span>`;
            }

            // Process line
            let result = escapeHtml(line);

            // List items (dash at start)
            result = result.replace(/^(\s*)(- )/, '$1<span class="yaml-dash">$2</span>');

            // Keys (word followed by colon)
            result = result.replace(/^(\s*)([a-zA-Z_][a-zA-Z0-9_]*)(:)/, '$1<span class="yaml-key">$2</span>$3');

            // Quoted strings
            result = result.replace(/"([^"]*)"/g, '<span class="yaml-string">"$1"</span>');
            result = result.replace(/'([^']*)'/g, '<span class="yaml-string">\'$1\'</span>');

            // Booleans
            result = result.replace(/:\s+(true|false)(\s|$)/gi, ': <span class="yaml-bool">$1</span>$2');

            // Numbers (after colon)
            result = result.replace(/:\s+(\d+\.?\d*)(\s|$)/g, ': <span class="yaml-number">$1</span>$2');

            return result;
        })
        .join('\n');
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Close modal on Escape key
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeYamlModal();
        closeRunModal();
    }
});

// Run Modal
let runModalEventSource = null;
let runModalAutoScroll = true;
let runModalStartTime = null;
let runModalTimeInterval = null;
let runModalStats = { passed: 0, failed: 0, skipped: 0, total: 0 };

function showRunModal(runId, target, pattern) {
    const modal = document.getElementById('run-modal');

    // Reset state
    runModalStats = { passed: 0, failed: 0, skipped: 0, total: 0 };
    runModalStartTime = Date.now();
    runModalAutoScroll = true;

    // Update UI
    document.getElementById('run-modal-id').textContent = runId.substring(0, 8) + '...';
    document.getElementById('run-modal-target').textContent = target || '-';
    document.getElementById('run-modal-pattern').textContent = pattern || 'All tests';
    document.getElementById('run-modal-status').textContent = 'RUNNING';
    document.getElementById('run-modal-status').className = 'run-status running';

    document.getElementById('run-modal-passed').textContent = '0';
    document.getElementById('run-modal-failed').textContent = '0';
    document.getElementById('run-modal-skipped').textContent = '0';
    document.getElementById('run-modal-total').textContent = '0';
    document.getElementById('run-modal-time').textContent = '0:00';
    document.getElementById('run-modal-progress-fill').style.width = '0%';

    document.getElementById('run-modal-log').innerHTML = `
        <div class="run-log-empty" id="run-modal-empty">
            <div class="spinner"></div>
            <div>Waiting for test results...</div>
        </div>
    `;
    document.getElementById('run-modal-complete').textContent = '';
    document.getElementById('run-modal-complete').className = 'run-complete-msg';
    document.getElementById('run-autoscroll-btn').textContent = 'Auto-scroll: ON';

    // Start time updater
    if (runModalTimeInterval) clearInterval(runModalTimeInterval);
    runModalTimeInterval = setInterval(updateRunModalTime, 1000);

    // Show modal
    modal.classList.add('active');

    // Connect to stream
    connectRunModalStream(runId);
}

async function showRunModalFromHistory(runId) {
    const modal = document.getElementById('run-modal');

    // Reset state
    runModalStats = { passed: 0, failed: 0, skipped: 0, total: 0 };
    runModalAutoScroll = true;

    // Show modal with loading state
    document.getElementById('run-modal-id').textContent = runId.substring(0, 8) + '...';
    document.getElementById('run-modal-target').textContent = 'Loading...';
    document.getElementById('run-modal-pattern').textContent = '';
    document.getElementById('run-modal-status').textContent = 'LOADING';
    document.getElementById('run-modal-status').className = 'run-status pending';
    document.getElementById('run-modal-log').innerHTML = `
        <div class="run-log-empty">
            <div class="spinner"></div>
            <div>Loading results...</div>
        </div>
    `;
    document.getElementById('run-modal-complete').textContent = '';
    document.getElementById('run-autoscroll-btn').textContent = 'Auto-scroll: ON';

    modal.classList.add('active');

    try {
        const response = await fetch(`${API_BASE}/runs/${runId}`);
        const data = await response.json();

        // Update header
        document.getElementById('run-modal-target').textContent = data.target || '-';
        document.getElementById('run-modal-pattern').textContent = data.pattern || 'All tests';
        document.getElementById('run-modal-status').textContent = data.status.toUpperCase();
        document.getElementById('run-modal-status').className = `run-status ${data.status}`;

        // Update stats
        runModalStats.passed = data.pass_count || 0;
        runModalStats.failed = data.fail_count || 0;
        runModalStats.skipped = data.skip_count || 0;
        runModalStats.total = data.total_count || 0;
        updateRunModalStats();

        // Calculate duration
        if (data.started_at) {
            runModalStartTime = new Date(data.started_at).getTime();
            if (data.completed_at) {
                const endTime = new Date(data.completed_at).getTime();
                const elapsed = Math.round((endTime - runModalStartTime) / 1000);
                document.getElementById('run-modal-time').textContent = formatTime(elapsed);
            } else {
                // Still running, start timer
                runModalTimeInterval = setInterval(updateRunModalTime, 1000);
            }
        }

        // Show results
        const logContainer = document.getElementById('run-modal-log');
        if (data.results && data.results.length > 0) {
            logContainer.innerHTML = '';
            data.results.forEach(result => addRunLogEntry(result));
        } else {
            logContainer.innerHTML = '<div class="run-log-empty">No results available</div>';
        }

        // Set progress
        document.getElementById('run-modal-progress-fill').style.width = '100%';

        // Show completion message if done
        if (data.status === 'completed' || data.status === 'failed') {
            showRunCompleteMessage();
        } else if (data.status === 'running') {
            // Connect to stream for live updates
            connectRunModalStream(runId);
        }

    } catch (error) {
        document.getElementById('run-modal-log').innerHTML = `
            <div class="run-log-empty">
                <div style="color: var(--error-color)">Error loading run: ${error.message}</div>
            </div>
        `;
    }
}

function connectRunModalStream(runId) {
    if (runModalEventSource) {
        runModalEventSource.close();
    }

    runModalEventSource = new EventSource(`${API_BASE}/runs/${runId}/stream`);

    runModalEventSource.addEventListener('result', (event) => {
        const result = JSON.parse(event.data);

        // Hide empty state
        const emptyEl = document.getElementById('run-modal-empty');
        if (emptyEl) emptyEl.style.display = 'none';

        // Update stats
        runModalStats.total++;
        if (result.status === 'passed') runModalStats.passed++;
        else if (result.status === 'failed') runModalStats.failed++;
        else if (result.status === 'skipped') runModalStats.skipped++;

        updateRunModalStats();
        addRunLogEntry(result);
    });

    runModalEventSource.addEventListener('done', () => {
        runModalEventSource.close();
        runModalEventSource = null;

        if (runModalTimeInterval) {
            clearInterval(runModalTimeInterval);
            runModalTimeInterval = null;
        }

        document.getElementById('run-modal-status').textContent = runModalStats.failed > 0 ? 'FAILED' : 'COMPLETED';
        document.getElementById('run-modal-status').className = `run-status ${runModalStats.failed > 0 ? 'failed' : 'completed'}`;
        document.getElementById('run-modal-progress-fill').style.width = '100%';

        showRunCompleteMessage();
        loadRuns(); // Refresh list
    });

    runModalEventSource.onerror = () => {
        if (runModalEventSource) {
            runModalEventSource.close();
            runModalEventSource = null;
        }
    };
}

function updateRunModalStats() {
    document.getElementById('run-modal-passed').textContent = runModalStats.passed;
    document.getElementById('run-modal-failed').textContent = runModalStats.failed;
    document.getElementById('run-modal-skipped').textContent = runModalStats.skipped;
    document.getElementById('run-modal-total').textContent = runModalStats.total;

    // Update progress (estimate based on total if we don't know expected)
    const progress = runModalStats.total > 0 ? Math.min(runModalStats.total * 2, 95) : 0;
    document.getElementById('run-modal-progress-fill').style.width = `${progress}%`;
}

function updateRunModalTime() {
    if (!runModalStartTime) return;
    const elapsed = Math.round((Date.now() - runModalStartTime) / 1000);
    document.getElementById('run-modal-time').textContent = formatTime(elapsed);
}

function formatTime(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function addRunLogEntry(result) {
    const container = document.getElementById('run-modal-log');
    const entry = document.createElement('div');
    entry.className = `run-log-entry ${result.status}`;

    const time = new Date().toLocaleTimeString('en-US', { hour12: false });

    entry.innerHTML = `
        <span class="run-log-time">${time}</span>
        <span class="run-log-status ${result.status}">${result.status.toUpperCase()}</span>
        <div class="run-log-test">
            <span class="run-log-test-id">${result.test_id}</span>
            <span class="run-log-test-name">${result.test_name || ''}</span>
            ${result.error ? `<div class="run-log-error">${result.error}</div>` : ''}
        </div>
        <span class="run-log-duration">${result.duration || ''}</span>
    `;

    container.appendChild(entry);

    if (runModalAutoScroll) {
        container.scrollTop = container.scrollHeight;
    }
}

function showRunCompleteMessage() {
    const msg = document.getElementById('run-modal-complete');
    const success = runModalStats.failed === 0;

    msg.className = `run-complete-msg ${success ? 'success' : 'failure'}`;
    msg.textContent = success
        ? `All ${runModalStats.passed} tests passed` + (runModalStats.skipped > 0 ? ` (${runModalStats.skipped} skipped)` : '')
        : `${runModalStats.failed} failed, ${runModalStats.passed} passed` + (runModalStats.skipped > 0 ? `, ${runModalStats.skipped} skipped` : '');
}

function closeRunModal(event) {
    if (event && event.target !== event.currentTarget) return;

    document.getElementById('run-modal').classList.remove('active');

    if (runModalEventSource) {
        runModalEventSource.close();
        runModalEventSource = null;
    }

    if (runModalTimeInterval) {
        clearInterval(runModalTimeInterval);
        runModalTimeInterval = null;
    }
}

function toggleRunAutoScroll() {
    runModalAutoScroll = !runModalAutoScroll;
    document.getElementById('run-autoscroll-btn').textContent = `Auto-scroll: ${runModalAutoScroll ? 'ON' : 'OFF'}`;
}

function clearRunLog() {
    document.getElementById('run-modal-log').innerHTML = '';
}
