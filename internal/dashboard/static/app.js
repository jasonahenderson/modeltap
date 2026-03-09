// modeltap Dashboard - Vanilla JS Application
(function () {
  "use strict";

  // === State ===
  const state = {
    page: 1,
    limit: 50,
    total: 0,
    filters: {},
    selectedId: null,
    autoRefresh: false,
    autoRefreshTimer: null,
  };

  // === DOM Refs ===
  const $ = (sel) => document.querySelector(sel);
  const $$ = (sel) => document.querySelectorAll(sel);

  const els = {
    tableBody: $("#log-table-body"),
    pageInfo: $("#page-info"),
    btnPrev: $("#btn-prev"),
    btnNext: $("#btn-next"),
    btnApply: $("#btn-apply-filters"),
    btnClear: $("#btn-clear-filters"),
    btnCloseDetail: $("#btn-close-detail"),
    detailPanel: $("#detail-panel"),
    detailTitle: $("#detail-title"),
    detailContent: $("#detail-content"),
    filterProvider: $("#filter-provider"),
    filterModel: $("#filter-model"),
    filterStatus: $("#filter-status"),
    filterSince: $("#filter-since"),
    filterUntil: $("#filter-until"),
    autoRefresh: $("#auto-refresh"),
    themeToggle: $("#themeToggle"),
  };

  // === Theme ===
  function initTheme() {
    const saved = localStorage.getItem("modeltap-theme");
    if (saved === "dark") {
      document.body.setAttribute("data-theme", "dark");
    }
  }

  function toggleTheme() {
    const isDark = document.body.getAttribute("data-theme") === "dark";
    if (isDark) {
      document.body.removeAttribute("data-theme");
      localStorage.setItem("modeltap-theme", "light");
    } else {
      document.body.setAttribute("data-theme", "dark");
      localStorage.setItem("modeltap-theme", "dark");
    }
  }

  // === Navigation ===
  function initNav() {
    $$(".nav-link").forEach((link) => {
      link.addEventListener("click", (e) => {
        e.preventDefault();
        const view = link.dataset.view;
        $$(".nav-link").forEach((l) => {
          l.classList.remove("active");
          l.removeAttribute("aria-current");
        });
        link.classList.add("active");
        link.setAttribute("aria-current", "page");
        $$(".view").forEach((v) => {
          v.classList.remove("active");
          v.hidden = true;
        });
        const viewEl = $(`#view-${view}`);
        viewEl.classList.add("active");
        viewEl.hidden = false;

        if (view === "metrics" && window.metricsView) {
          window.metricsView.init();
        }
        if (view === "status") {
          loadStatus();
        }
      });
    });
  }

  // === Filters ===
  function gatherFilters() {
    const filters = {};
    const provider = els.filterProvider.value;
    const model = els.filterModel.value.trim();
    const status = els.filterStatus.value;
    const since = els.filterSince.value;
    const until = els.filterUntil.value;

    if (provider) filters.provider = provider;
    if (model) filters.model = model;
    if (status) filters.status = status;
    if (since) filters.since = new Date(since).toISOString();
    if (until) filters.until = new Date(until).toISOString();

    return filters;
  }

  function clearFilters() {
    els.filterProvider.value = "";
    els.filterModel.value = "";
    els.filterStatus.value = "";
    els.filterSince.value = "";
    els.filterUntil.value = "";
    state.filters = {};
    state.page = 1;
    fetchLogs();
  }

  function applyFilters() {
    state.filters = gatherFilters();
    state.page = 1;
    fetchLogs();
  }

  // === API ===
  async function fetchLogs() {
    const params = new URLSearchParams();
    params.set("limit", state.limit);
    params.set("offset", (state.page - 1) * state.limit);

    for (const [k, v] of Object.entries(state.filters)) {
      params.set(k, v);
    }

    try {
      const resp = await fetch(`/api/logs?${params}`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const json = await resp.json();
      state.total = json.total || 0;
      renderTable(json.data || []);
      renderPagination();
    } catch (err) {
      els.tableBody.innerHTML = `<tr><td colspan="9" class="empty-state">Error loading logs: ${escapeHtml(err.message)}</td></tr>`;
    }
  }

  async function fetchLogDetail(id) {
    try {
      const resp = await fetch(`/api/logs/${encodeURIComponent(id)}`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      return await resp.json();
    } catch (err) {
      return { error: err.message };
    }
  }

  async function loadStatus() {
    const content = $("#status-content");
    try {
      const resp = await fetch("/api/status");
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      content.innerHTML = renderStatus(data);
    } catch (err) {
      content.innerHTML = `<p class="placeholder-text">Error loading status: ${escapeHtml(err.message)}</p>`;
    }
  }

  // === Rendering ===
  function escapeHtml(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  function truncateId(id) {
    if (!id) return "";
    return id.length > 8 ? id.slice(0, 8) + "\u2026" : id;
  }

  function formatTimestamp(ts) {
    if (!ts) return "";
    try {
      const d = new Date(ts);
      return d.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });
    } catch {
      return ts;
    }
  }

  function formatCost(cost) {
    if (cost == null || cost === 0) return "-";
    if (cost < 0.01) return "$" + cost.toFixed(6);
    return "$" + cost.toFixed(4);
  }

  function formatLatency(ms) {
    if (ms == null) return "-";
    if (ms < 1000) return ms + "ms";
    return (ms / 1000).toFixed(2) + "s";
  }

  function statusClass(code) {
    if (code >= 200 && code < 300) return "status-2xx";
    if (code >= 400 && code < 500) return "status-4xx";
    if (code >= 500) return "status-5xx";
    return "";
  }

  function renderTable(data) {
    if (!data || data.length === 0) {
      els.tableBody.innerHTML = '<tr><td colspan="9" class="empty-state">No log entries found.</td></tr>';
      return;
    }

    els.tableBody.innerHTML = data
      .map(
        (row) => `
      <tr data-id="${escapeHtml(row.id)}" role="button" tabindex="0" aria-label="View details for request ${escapeHtml(truncateId(row.id))}">
        <td class="id-cell" title="${escapeHtml(row.id)}">${escapeHtml(truncateId(row.id))}</td>
        <td>${escapeHtml(formatTimestamp(row.timestamp))}</td>
        <td>${escapeHtml(row.provider || "-")}</td>
        <td>${escapeHtml(row.model || "-")}</td>
        <td><span class="status-badge ${statusClass(row.response_status)}">${escapeHtml(String(row.response_status || "-"))}</span></td>
        <td>${row.input_tokens != null ? row.input_tokens.toLocaleString() : "-"}</td>
        <td>${row.output_tokens != null ? row.output_tokens.toLocaleString() : "-"}</td>
        <td>${formatCost(row.estimated_cost_usd)}</td>
        <td>${formatLatency(row.latency_ms)}</td>
      </tr>`
      )
      .join("");

    // Attach click handlers
    els.tableBody.querySelectorAll("tr[data-id]").forEach((tr) => {
      tr.addEventListener("click", () => showDetail(tr.dataset.id));
      tr.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          showDetail(tr.dataset.id);
        }
      });
    });
  }

  function renderPagination() {
    const totalPages = Math.max(1, Math.ceil(state.total / state.limit));
    els.pageInfo.textContent = `Page ${state.page} of ${totalPages} (${state.total} entries)`;
    els.btnPrev.disabled = state.page <= 1;
    els.btnNext.disabled = state.page >= totalPages;
  }

  async function showDetail(id) {
    // Highlight selected row
    els.tableBody.querySelectorAll("tr").forEach((tr) => tr.classList.remove("selected"));
    const row = els.tableBody.querySelector(`tr[data-id="${CSS.escape(id)}"]`);
    if (row) row.classList.add("selected");

    state.selectedId = id;
    els.detailPanel.hidden = false;
    els.detailTitle.textContent = `Request ${truncateId(id)}`;
    els.detailContent.innerHTML = '<p class="placeholder-text">Loading...</p>';

    const data = await fetchLogDetail(id);
    if (data.error) {
      els.detailContent.innerHTML = `<p class="placeholder-text">Error: ${escapeHtml(data.error)}</p>`;
      return;
    }

    els.detailContent.innerHTML = renderDetail(data);
  }

  function renderDetail(d) {
    return `
      <div class="detail-section">
        <h4>Overview</h4>
        <dl class="detail-meta">
          <dt>ID</dt><dd style="font-family:monospace">${escapeHtml(d.id)}</dd>
          <dt>Timestamp</dt><dd>${escapeHtml(d.timestamp)}</dd>
          <dt>Provider</dt><dd>${escapeHtml(d.provider || "-")}</dd>
          <dt>Model</dt><dd>${escapeHtml(d.model || "-")}</dd>
          <dt>Method</dt><dd>${escapeHtml(d.method || "-")}</dd>
          <dt>URL</dt><dd style="word-break:break-all">${escapeHtml(d.url || "-")}</dd>
          <dt>Status</dt><dd><span class="status-badge ${statusClass(d.response_status)}">${escapeHtml(String(d.response_status || "-"))}</span></dd>
          <dt>Latency</dt><dd>${formatLatency(d.latency_ms)}</dd>
          <dt>Input Tokens</dt><dd>${d.input_tokens != null ? d.input_tokens.toLocaleString() : "-"}</dd>
          <dt>Output Tokens</dt><dd>${d.output_tokens != null ? d.output_tokens.toLocaleString() : "-"}</dd>
          <dt>Estimated Cost</dt><dd>${formatCost(d.estimated_cost_usd)}</dd>
        </dl>
      </div>
      <div class="detail-section">
        <h4>Request Headers</h4>
        <div class="json-container">${highlightJSON(d.request_headers)}</div>
      </div>
      <div class="detail-section">
        <h4>Request Body</h4>
        <div class="json-container">${highlightJSON(d.request_body)}</div>
      </div>
      <div class="detail-section">
        <h4>Response Headers</h4>
        <div class="json-container">${highlightJSON(d.response_headers)}</div>
      </div>
      <div class="detail-section">
        <h4>Response Body</h4>
        <div class="json-container">${highlightJSON(d.response_body)}</div>
      </div>
    `;
  }

  // === JSON Syntax Highlighting ===
  function highlightJSON(str) {
    if (!str) return '<span class="json-null">empty</span>';

    // Try to parse and pretty-print
    let formatted;
    try {
      const parsed = JSON.parse(str);
      formatted = JSON.stringify(parsed, null, 2);
    } catch {
      // Not valid JSON, display as-is escaped
      return escapeHtml(str);
    }

    // Tokenize and highlight
    return formatted.replace(
      /("(?:\\.|[^"\\])*")\s*:/g,
      (match, key) => `<span class="json-key">${escapeHtml(key)}</span>:`
    ).replace(
      /:\s*("(?:\\.|[^"\\])*")/g,
      (match, val) => ': <span class="json-string">' + escapeHtml(val) + "</span>"
    ).replace(
      /:\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
      (match, num) => ': <span class="json-number">' + escapeHtml(num) + "</span>"
    ).replace(
      /:\s*(true|false)/g,
      (match, val) => ': <span class="json-boolean">' + escapeHtml(val) + "</span>"
    ).replace(
      /:\s*(null)/g,
      (match, val) => ': <span class="json-null">' + escapeHtml(val) + "</span>"
    ).replace(
      // Handle values in arrays (not preceded by colon)
      /(?<=[\[,\n]\s*)("(?:\\.|[^"\\])*")(?=\s*[,\]\n])/g,
      (match) => '<span class="json-string">' + escapeHtml(match) + "</span>"
    ).replace(
      /(?<=[\[,\n]\s*)(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)(?=\s*[,\]\n])/g,
      (match) => '<span class="json-number">' + escapeHtml(match) + "</span>"
    ).replace(
      /(?<=[\[,\n]\s*)(true|false)(?=\s*[,\]\n])/g,
      (match) => '<span class="json-boolean">' + escapeHtml(match) + "</span>"
    ).replace(
      /(?<=[\[,\n]\s*)(null)(?=\s*[,\]\n])/g,
      (match) => '<span class="json-null">' + escapeHtml(match) + "</span>"
    );
  }

  // === Status Page ===
  function renderStatus(data) {
    return `
      <div class="status-grid">
        <div class="status-card">
          <h4>Proxy</h4>
          <dl>
            <dt>Port</dt><dd>${escapeHtml(String(data.proxy?.port || "-"))}</dd>
            <dt>Upstream</dt><dd>${escapeHtml(data.proxy?.upstream || "-")}</dd>
          </dl>
        </div>
        <div class="status-card">
          <h4>Database</h4>
          <dl>
            <dt>Records</dt><dd>${escapeHtml(data.database?.records != null ? data.database.records.toLocaleString() : "-")}</dd>
          </dl>
        </div>
        <div class="status-card">
          <h4>Retention</h4>
          <dl>
            <dt>Days</dt><dd>${escapeHtml(String(data.retention?.days || "-"))}</dd>
          </dl>
        </div>
      </div>
    `;
  }

  // === Auto-Refresh ===
  function setAutoRefresh(enabled) {
    state.autoRefresh = enabled;
    if (state.autoRefreshTimer) {
      clearInterval(state.autoRefreshTimer);
      state.autoRefreshTimer = null;
    }
    if (enabled) {
      state.autoRefreshTimer = setInterval(() => {
        fetchLogs();
      }, 5000);
    }
  }

  // === Init ===
  function init() {
    initTheme();
    initNav();

    // Event listeners
    els.btnApply.addEventListener("click", applyFilters);
    els.btnClear.addEventListener("click", clearFilters);
    els.btnCloseDetail.addEventListener("click", () => {
      els.detailPanel.hidden = true;
      state.selectedId = null;
      els.tableBody.querySelectorAll("tr").forEach((tr) => tr.classList.remove("selected"));
    });

    els.btnPrev.addEventListener("click", () => {
      if (state.page > 1) {
        state.page--;
        fetchLogs();
      }
    });

    els.btnNext.addEventListener("click", () => {
      const totalPages = Math.ceil(state.total / state.limit);
      if (state.page < totalPages) {
        state.page++;
        fetchLogs();
      }
    });

    els.autoRefresh.addEventListener("change", (e) => {
      setAutoRefresh(e.target.checked);
    });

    els.themeToggle.addEventListener("click", toggleTheme);

    // Allow Enter in filter inputs to apply
    [els.filterModel, els.filterSince, els.filterUntil].forEach((el) => {
      el.addEventListener("keydown", (e) => {
        if (e.key === "Enter") applyFilters();
      });
    });

    [els.filterProvider, els.filterStatus].forEach((el) => {
      el.addEventListener("change", applyFilters);
    });

    // Initial load
    fetchLogs();
  }

  // Start when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
