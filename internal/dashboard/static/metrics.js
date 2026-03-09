// Metrics view for modeltap dashboard.
// Fetches /api/metrics and renders summary cards, bar chart, and data table.
(function () {
    "use strict";

    // -- Formatting helpers --------------------------------------------------

    function fmtNum(n) {
        if (n == null) return "0";
        return Number(n).toLocaleString("en-US");
    }

    function fmtCost(n) {
        if (n == null || n === 0) return "$0.00";
        if (n < 0.01) return "$" + n.toFixed(4);
        return "$" + Number(n).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 4 });
    }

    function fmtLatency(ms) {
        if (ms == null || ms === 0) return "0 ms";
        if (ms >= 1000) return (ms / 1000).toFixed(1) + " s";
        return fmtNum(ms) + " ms";
    }

    function fmtTokens(n) {
        if (n == null) return "0";
        if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
        if (n >= 1_000) return (n / 1_000).toFixed(1) + "K";
        return fmtNum(n);
    }

    // -- DOM helpers ---------------------------------------------------------

    function el(tag, attrs, children) {
        var e = document.createElement(tag);
        if (attrs) {
            Object.keys(attrs).forEach(function (k) {
                if (k === "className") e.className = attrs[k];
                else if (k === "textContent") e.textContent = attrs[k];
                else if (k === "innerHTML") e.innerHTML = attrs[k];
                else if (k.startsWith("on")) e.addEventListener(k.slice(2).toLowerCase(), attrs[k]);
                else e.setAttribute(k, attrs[k]);
            });
        }
        if (children) {
            children.forEach(function (c) {
                if (typeof c === "string") e.appendChild(document.createTextNode(c));
                else if (c) e.appendChild(c);
            });
        }
        return e;
    }

    // -- State ---------------------------------------------------------------

    var state = {
        timeRange: "7d",
        groupBy: "day",
        chartMetric: "requests",
        customSince: "",
        customUntil: "",
        data: [],
        loading: false,
        error: null,
    };

    // -- API -----------------------------------------------------------------

    function buildParams() {
        var params = new URLSearchParams();
        params.set("group_by", state.groupBy);

        var now = new Date();
        var since;
        switch (state.timeRange) {
            case "today":
                since = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                break;
            case "7d":
                since = new Date(now.getTime() - 7 * 24 * 3600 * 1000);
                break;
            case "30d":
                since = new Date(now.getTime() - 30 * 24 * 3600 * 1000);
                break;
            case "custom":
                if (state.customSince) params.set("since", new Date(state.customSince).toISOString());
                if (state.customUntil) params.set("until", new Date(state.customUntil + "T23:59:59").toISOString());
                return params;
        }
        if (since) params.set("since", since.toISOString());
        return params;
    }

    function fetchMetrics() {
        state.loading = true;
        state.error = null;
        render();

        var params = buildParams();
        fetch("/api/metrics?" + params.toString())
            .then(function (res) {
                if (!res.ok) throw new Error("HTTP " + res.status);
                return res.json();
            })
            .then(function (body) {
                state.data = body.data || [];
                state.loading = false;
                render();
            })
            .catch(function (err) {
                state.error = err.message;
                state.loading = false;
                render();
            });
    }

    // -- Aggregation ---------------------------------------------------------

    function computeSummary(data) {
        var totalRequests = 0;
        var totalTokens = 0;
        var totalCost = 0;
        var totalLatencyWeighted = 0;
        for (var i = 0; i < data.length; i++) {
            var d = data[i];
            totalRequests += d.request_count || 0;
            totalTokens += (d.input_tokens || 0) + (d.output_tokens || 0);
            totalCost += d.estimated_cost || 0;
            totalLatencyWeighted += (d.avg_latency_ms || 0) * (d.request_count || 0);
        }
        var avgLatency = totalRequests > 0 ? Math.round(totalLatencyWeighted / totalRequests) : 0;
        return {
            totalRequests: totalRequests,
            totalTokens: totalTokens,
            totalCost: totalCost,
            avgLatency: avgLatency,
        };
    }

    // -- Renderers -----------------------------------------------------------

    function renderControls() {
        var timeSelect = el("select", { id: "time-range" }, [
            el("option", { value: "today", textContent: "Today" }),
            el("option", { value: "7d", textContent: "Last 7 Days" }),
            el("option", { value: "30d", textContent: "Last 30 Days" }),
            el("option", { value: "custom", textContent: "Custom Range" }),
        ]);
        timeSelect.value = state.timeRange;
        timeSelect.addEventListener("change", function () {
            state.timeRange = this.value;
            render();
            fetchMetrics();
        });

        var groupSelect = el("select", { id: "group-by" }, [
            el("option", { value: "day", textContent: "Day" }),
            el("option", { value: "hour", textContent: "Hour" }),
            el("option", { value: "provider", textContent: "Provider" }),
            el("option", { value: "model", textContent: "Model" }),
        ]);
        groupSelect.value = state.groupBy;
        groupSelect.addEventListener("change", function () {
            state.groupBy = this.value;
            fetchMetrics();
        });

        var customRange = el("div", { className: "custom-range" + (state.timeRange === "custom" ? " visible" : "") }, [
            el("input", { type: "date", id: "custom-since", value: state.customSince }),
            el("span", { textContent: "to" }),
            el("input", { type: "date", id: "custom-until", value: state.customUntil }),
        ]);
        customRange.querySelector("#custom-since").addEventListener("change", function () {
            state.customSince = this.value;
            fetchMetrics();
        });
        customRange.querySelector("#custom-until").addEventListener("change", function () {
            state.customUntil = this.value;
            fetchMetrics();
        });

        return el("div", { className: "metrics-controls" }, [
            el("div", { className: "time-range-group" }, [
                el("label", { textContent: "Time Range" }),
                timeSelect,
                customRange,
            ]),
            el("div", { className: "group-by-group" }, [
                el("label", { textContent: "Group By" }),
                groupSelect,
            ]),
        ]);
    }

    function renderSummary(summary) {
        return el("div", { className: "metrics-summary" }, [
            el("div", { className: "summary-card requests" }, [
                el("div", { className: "card-label", textContent: "Total Requests" }),
                el("div", { className: "card-value", textContent: fmtNum(summary.totalRequests) }),
            ]),
            el("div", { className: "summary-card tokens" }, [
                el("div", { className: "card-label", textContent: "Total Tokens" }),
                el("div", { className: "card-value", textContent: fmtTokens(summary.totalTokens) }),
            ]),
            el("div", { className: "summary-card cost" }, [
                el("div", { className: "card-label", textContent: "Total Cost" }),
                el("div", { className: "card-value", textContent: fmtCost(summary.totalCost) }),
            ]),
            el("div", { className: "summary-card latency" }, [
                el("div", { className: "card-label", textContent: "Avg Latency" }),
                el("div", { className: "card-value", textContent: fmtLatency(summary.avgLatency) }),
            ]),
        ]);
    }

    function renderChart(data) {
        var metric = state.chartMetric;
        var metricKey, metricFmt, cssClass;
        switch (metric) {
            case "requests":
                metricKey = "request_count"; metricFmt = fmtNum; cssClass = "requests"; break;
            case "tokens":
                metricKey = function (d) { return (d.input_tokens || 0) + (d.output_tokens || 0); };
                metricFmt = fmtTokens; cssClass = "tokens"; break;
            case "cost":
                metricKey = "estimated_cost"; metricFmt = fmtCost; cssClass = "cost"; break;
            case "latency":
                metricKey = "avg_latency_ms"; metricFmt = fmtLatency; cssClass = "latency"; break;
        }

        // Extract values
        var items = data.map(function (d) {
            var val = typeof metricKey === "function" ? metricKey(d) : (d[metricKey] || 0);
            var label = d.period || d.provider || d.model || "-";
            return { label: label, value: val };
        });

        var maxVal = Math.max.apply(null, items.map(function (i) { return i.value; }));
        if (maxVal === 0) maxVal = 1;

        // Limit to 20 bars
        var displayItems = items.slice(0, 20);

        var buttons = ["requests", "tokens", "cost", "latency"].map(function (m) {
            return el("button", {
                className: "chart-metric-btn" + (m === metric ? " active" : ""),
                textContent: m.charAt(0).toUpperCase() + m.slice(1),
                onClick: function () {
                    state.chartMetric = m;
                    render();
                },
            });
        });

        var bars = displayItems.map(function (item) {
            var pct = Math.max(1, (item.value / maxVal) * 100);
            var fill = el("div", { className: "chart-bar-fill " + cssClass });
            fill.style.width = pct + "%";
            return el("div", { className: "chart-bar-row" }, [
                el("div", { className: "chart-bar-label", textContent: item.label }),
                el("div", { className: "chart-bar-track" }, [fill]),
                el("div", { className: "chart-bar-value", textContent: metricFmt(item.value) }),
            ]);
        });

        return el("div", { className: "metrics-chart" }, [
            el("h3", { textContent: "Metrics by " + state.groupBy }),
            el("div", { className: "chart-metric-selector" }, buttons),
        ].concat(bars));
    }

    function renderTable(data) {
        var thead = el("thead", null, [
            el("tr", null, [
                el("th", { textContent: "Period" }),
                el("th", { textContent: "Provider" }),
                el("th", { textContent: "Model" }),
                el("th", { className: "num", textContent: "Requests" }),
                el("th", { className: "num", textContent: "Input Tokens" }),
                el("th", { className: "num", textContent: "Output Tokens" }),
                el("th", { className: "num", textContent: "Cost" }),
                el("th", { className: "num", textContent: "Avg Latency" }),
                el("th", { className: "num", textContent: "Errors" }),
            ]),
        ]);

        var rows = data.map(function (d) {
            return el("tr", null, [
                el("td", { textContent: d.period || "-" }),
                el("td", { textContent: d.provider || "-" }),
                el("td", { textContent: d.model || "-" }),
                el("td", { className: "num", textContent: fmtNum(d.request_count) }),
                el("td", { className: "num", textContent: fmtNum(d.input_tokens) }),
                el("td", { className: "num", textContent: fmtNum(d.output_tokens) }),
                el("td", { className: "num", textContent: fmtCost(d.estimated_cost) }),
                el("td", { className: "num", textContent: fmtLatency(d.avg_latency_ms) }),
                el("td", { className: "num", textContent: fmtNum(d.error_count) }),
            ]);
        });

        var tbody = el("tbody", null, rows);
        var table = el("table", { className: "metrics-table" }, [thead, tbody]);
        return el("div", { className: "metrics-table-wrap" }, [table]);
    }

    // -- Main render ---------------------------------------------------------

    function render() {
        var container = document.getElementById("metrics-view");
        if (!container) return;
        container.innerHTML = "";

        container.appendChild(renderControls());

        if (state.loading) {
            container.appendChild(el("div", { className: "metrics-loading", textContent: "Loading metrics" }));
            return;
        }

        if (state.error) {
            container.appendChild(el("div", { className: "metrics-error", textContent: "Error: " + state.error }));
            return;
        }

        var data = state.data;
        if (!data || data.length === 0) {
            container.appendChild(el("div", { className: "metrics-empty", textContent: "No metrics data for this time range." }));
            return;
        }

        var summary = computeSummary(data);
        container.appendChild(renderSummary(summary));
        container.appendChild(renderChart(data));
        container.appendChild(renderTable(data));
    }

    // -- Public API for integration with main dashboard ----------------------

    window.metricsView = {
        init: function () {
            fetchMetrics();
        },
        render: render,
        refresh: fetchMetrics,
    };

    // Auto-init if metrics-view element is already present (standalone mode).
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", function () {
            if (document.getElementById("metrics-view")) {
                window.metricsView.init();
            }
        });
    } else {
        if (document.getElementById("metrics-view")) {
            window.metricsView.init();
        }
    }
})();
