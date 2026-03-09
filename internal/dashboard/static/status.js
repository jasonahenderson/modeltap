// Status view — fetches /api/status and renders status cards.
// Auto-refreshes every 10 seconds.

(function () {
    'use strict';

    var REFRESH_INTERVAL = 10000; // 10 seconds
    var refreshTimer = null;

    // Format a number with comma separators (e.g. 1234567 -> "1,234,567").
    function formatNumber(n) {
        if (n == null) return '0';
        return Number(n).toLocaleString();
    }

    // Truncate a URL for display, keeping protocol + host.
    function truncateURL(url) {
        if (!url) return '-';
        try {
            var u = new URL(url);
            return u.protocol + '//' + u.host;
        } catch (e) {
            return url;
        }
    }

    // Show or hide the error banner.
    function showError(msg) {
        var el = document.getElementById('status-error');
        if (!el) return;
        if (msg) {
            el.textContent = msg;
            el.classList.add('visible');
        } else {
            el.classList.remove('visible');
        }
    }

    // Set loading state on refresh indicator.
    function setLoading(loading) {
        var el = document.getElementById('refresh-indicator');
        if (!el) return;
        if (loading) {
            el.classList.add('loading');
        } else {
            el.classList.remove('loading');
        }
        updateTimestamp();
    }

    // Update the "last refreshed" timestamp.
    function updateTimestamp() {
        var el = document.getElementById('refresh-time');
        if (!el) return;
        var now = new Date();
        el.textContent = 'Updated ' + now.toLocaleTimeString();
    }

    // Build the proxy card content.
    function renderProxy(proxy) {
        var container = document.getElementById('proxy-details');
        if (!container) return;

        var port = proxy.port || '-';
        var upstream = proxy.upstream || '-';

        container.innerHTML =
            '<ul class="kv-list">' +
            '<li class="kv-item">' +
            '<span class="kv-label">Status</span>' +
            '<span class="kv-value"><span class="status-dot running">Running</span></span>' +
            '</li>' +
            '<li class="kv-item">' +
            '<span class="kv-label">Port</span>' +
            '<span class="kv-value">' + escapeHTML(String(port)) + '</span>' +
            '</li>' +
            '<li class="kv-item">' +
            '<span class="kv-label">Upstream</span>' +
            '<span class="kv-value" title="' + escapeHTML(upstream) + '">' + escapeHTML(truncateURL(upstream)) + '</span>' +
            '</li>' +
            '</ul>';
    }

    // Build the database card content.
    function renderDatabase(db) {
        var container = document.getElementById('database-details');
        if (!container) return;

        var records = db.records != null ? db.records : 0;
        var path = db.path || null;

        var html =
            '<ul class="kv-list">' +
            '<li class="kv-item">' +
            '<span class="kv-label">Records</span>' +
            '<span class="kv-value">' + formatNumber(records) + '</span>' +
            '</li>';

        if (path) {
            html +=
                '<li class="kv-item">' +
                '<span class="kv-label">Path</span>' +
                '<span class="kv-value" title="' + escapeHTML(path) + '">' + escapeHTML(shortenPath(path)) + '</span>' +
                '</li>';
        }

        if (db.size) {
            html +=
                '<li class="kv-item">' +
                '<span class="kv-label">Size</span>' +
                '<span class="kv-value">' + escapeHTML(db.size) + '</span>' +
                '</li>';
        }

        html += '</ul>';
        container.innerHTML = html;
    }

    // Build the retention card content.
    function renderRetention(retention) {
        var container = document.getElementById('retention-details');
        if (!container) return;

        var days = retention.days != null ? retention.days : '-';

        container.innerHTML =
            '<ul class="kv-list">' +
            '<li class="kv-item">' +
            '<span class="kv-label">Retention Period</span>' +
            '<span class="kv-value">' + escapeHTML(String(days)) + ' days</span>' +
            '</li>' +
            '</ul>';
    }

    // Build the providers card content.
    function renderProviders(providers) {
        var container = document.getElementById('providers-details');
        if (!container) return;

        if (!providers || (Array.isArray(providers) && providers.length === 0) ||
            (typeof providers === 'object' && !Array.isArray(providers) && Object.keys(providers).length === 0)) {
            container.innerHTML = '<p class="empty-state">No providers configured</p>';
            return;
        }

        var html = '<ul class="provider-list">';

        if (Array.isArray(providers)) {
            for (var i = 0; i < providers.length; i++) {
                var p = providers[i];
                var name = typeof p === 'string' ? p : (p.name || p.provider || 'unknown');
                var upstream = typeof p === 'object' ? (p.upstream || '') : '';
                html += '<li class="provider-item">' +
                    escapeHTML(name) +
                    (upstream ? '<span class="upstream">' + escapeHTML(truncateURL(upstream)) + '</span>' : '') +
                    '</li>';
            }
        } else if (typeof providers === 'object') {
            var keys = Object.keys(providers);
            for (var j = 0; j < keys.length; j++) {
                var key = keys[j];
                var val = providers[key];
                var upstreamURL = typeof val === 'object' ? (val.upstream || '') : String(val);
                html += '<li class="provider-item">' +
                    escapeHTML(key) +
                    (upstreamURL ? '<span class="upstream">' + escapeHTML(truncateURL(upstreamURL)) + '</span>' : '') +
                    '</li>';
            }
        }

        html += '</ul>';
        container.innerHTML = html;
    }

    // Shorten a file path for display.
    function shortenPath(p) {
        if (!p || p.length < 40) return p;
        var parts = p.split('/');
        if (parts.length <= 3) return p;
        return '.../' + parts.slice(-3).join('/');
    }

    // Escape HTML to prevent XSS.
    function escapeHTML(str) {
        var div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }

    // Fetch status data and render.
    function fetchStatus() {
        setLoading(true);

        fetch('/api/status')
            .then(function (res) {
                if (!res.ok) throw new Error('HTTP ' + res.status);
                return res.json();
            })
            .then(function (data) {
                showError(null);
                renderProxy(data.proxy || {});
                renderDatabase(data.database || {});
                renderRetention(data.retention || {});
                renderProviders(data.providers || {});
                setLoading(false);
            })
            .catch(function (err) {
                showError('Failed to load status: ' + err.message);
                setLoading(false);
            });
    }

    // Initialize: fetch immediately, then set up auto-refresh.
    function init() {
        fetchStatus();
        refreshTimer = setInterval(fetchStatus, REFRESH_INTERVAL);
    }

    // Start when DOM is ready.
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
