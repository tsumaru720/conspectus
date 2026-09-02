/* Light/dark toggle, Chart.js bootstrap reading CSS variables, and small
 * progressive enhancements. Everything works without JS except the charts. */
(function () {
  "use strict";

  /* ---- Light/dark toggle ---- */
  var toggle = document.getElementById("theme-toggle");
  function applyTheme(t) {
    document.documentElement.setAttribute("data-theme", t);
    localStorage.setItem("theme", t);
    rerenderCharts();
  }
  if (toggle) {
    toggle.addEventListener("click", function () {
      var cur = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
      applyTheme(cur);
    });
  }

  /* ---- CSS-variable-driven chart palette ---- */
  function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }
  function palette() {
    var cols = [];
    for (var i = 1; i <= 8; i++) {
      var c = cssVar("--c" + i);
      if (c) cols.push(c);
    }
    return cols;
  }

  /* ---- Page identity + UI state in localStorage ----
   * How pages look (range, projections horizon/window, ledger search,
   * page size, asset/class scope) persists in the browser; app data stays
   * in the database. The server-visible subset is mirrored into the
   * conspectus_ui cookie so full-page GETs render the right window - URLs
   * never carry query strings. A scope picked on one page re-applies to
   * the other browse-picker pages; its path form (/analytics/asset/47)
   * wins while it is on the URL. */
  var PAGE = document.body.getAttribute("data-page") || "";
  var UI_STORE = "conspectus.ui";
  var UI_COOKIE = "conspectus_ui";
  var SERVER_KEYS = ["from", "to", "q", "horizon", "window", "per_page",
    "scope.asset_id", "scope.class_id"];
  function uiState() {
    try { return JSON.parse(localStorage.getItem(UI_STORE) || "{}") || {}; } catch (e) { return {}; }
  }
  function uiWrite(s) { localStorage.setItem(UI_STORE, JSON.stringify(s)); }
  function uiSave(key, val) {
    var s = uiState();
    if (val) s[key] = val; else delete s[key];
    uiWrite(s);
    syncUICookie();
  }
  /* Mirror the server-visible keys into the cookie (url-encoded params).
   * max-age keeps the memory across browser restarts. */
  function syncUICookie() {
    var s = uiState();
    var parts = [];
    SERVER_KEYS.forEach(function (k) {
      if (s[k]) parts.push(k + "=" + encodeURIComponent(s[k]));
    });
    document.cookie = UI_COOKIE + "=" + encodeURIComponent(parts.join("&")) + "; path=/; SameSite=Lax; max-age=31536000";
  }
  syncUICookie();
  // The log scale is gone from projections; drop its stored preference.
  if (uiState()["projections.scale"] !== undefined) uiSave("projections.scale", "");
  // Scope is remembered globally, not per page; drop the per-page keys.
  (function () {
    var s = uiState(), dropped = false;
    Object.keys(s).forEach(function (k) {
      if (k !== "scope.asset_id" && k !== "scope.class_id" && /\.(asset_id|class_id)$/.test(k)) {
        delete s[k];
        dropped = true;
      }
    });
    if (dropped) { uiWrite(s); syncUICookie(); }
  })();
  // A from/to that is not a YYYY-MM month fails the stats API on every
  // page, so anything else stored there is dropped.
  (function () {
    var s = uiState(), dropped = false;
    ["from", "to"].forEach(function (k) {
      if (s[k] !== undefined && !/^\d{4}-(0[1-9]|1[0-2])$/.test(s[k])) {
        delete s[k];
        dropped = true;
      }
    });
    if (dropped) { uiWrite(s); syncUICookie(); }
  })();
  // Month inputs accept free text where the browser has no native month
  // picker; the saved value must be a real YYYY-MM month.
  function monthInputOK(input) {
    if (!input) return true;
    var v = (input.value || "").trim();
    if (v === "" || /^\d{4}-(0[1-9]|1[0-2])$/.test(v)) {
      input.setCustomValidity("");
      return true;
    }
    input.setCustomValidity("Use a month in YYYY-MM format, e.g. 2026-08");
    return false;
  }
  function monthsOK(inputs) {
    var bad = inputs.filter(function (el) { return !monthInputOK(el); });
    if (bad.length) { bad[0].reportValidity(); return false; }
    return true;
  }
  // Landing on a path-scoped page (/asset/47, /breakdown/class/3, ...)
  // makes that scope the remembered one, however the page was reached.
  (function () {
    var m = window.location.pathname.match(/\/(asset|class)\/(\d+)(\/pay\/\d+)?$/);
    if (!m) return;
    uiSave("scope." + m[1] + "_id", m[2]);
    uiSave("scope." + (m[1] === "asset" ? "class_id" : "asset_id"), "");
  })();

  function chartDefaults() {
    var text = cssVar("--text-muted") || "#666";
    var border = cssVar("--border") || "#ddd";
    Chart.defaults.color = text;
    Chart.defaults.borderColor = border;
    Chart.defaults.font.family = "system-ui, sans-serif";
    // No animations anywhere: charts render in their final state (this also
    // covers hover/tooltip transitions, which inherit from the global).
    Chart.defaults.animation = false;
  }

  /* Legend keys for toggleable line charts: a solid block of the series
   * colour (the default key is a coloured outline around a transparent
   * box, which reads as two different colours for filled series). */
  function solidLegendLabels(chart) {
    if (chart.config.type !== "line" || typeof Chart.defaults.plugins.legend.labels.generateLabels !== "function") {
      return Chart.defaults.plugins.legend.labels.generateLabels(chart);
    }
    var items = Chart.defaults.plugins.legend.labels.generateLabels(chart);
    items.forEach(function (it) {
      if (it.strokeStyle) {
        it.fillStyle = it.strokeStyle;
        it.lineWidth = 1;
      }
    });
    return items;
  }

  /* ---- Name-derived pastel colours ----
   * Port of the v1 pastel_colour(): a series' colour is a deterministic
   * derivative of its name, so "Vanguard S&P 500 ETF" is always the same
   * colour on every chart. Falls back to the CSS palette cycle. */
  var pastelCache = {};
  function pastelRGB(name) {
    if (!name) return "";
    if (pastelCache[name]) return pastelCache[name];
    var baseRed = 100, baseGreen = 100, baseBlue = 100;
    var seed = 0;
    for (var i = 0; i < name.length; i++) {
      seed ^= name.charCodeAt(i);
    }
    var rand1 = Math.abs(Math.sin(seed++) * 10000) % 360;
    var rand2 = Math.abs(Math.sin(seed++) * 10000) % 360;
    var rand3 = Math.abs(Math.sin(seed++) * 10000) % 360;
    var rgb = Math.round((rand1 + baseRed) / 2) + "," +
              Math.round((rand2 + baseGreen) / 2) + "," +
              Math.round((rand3 + baseBlue) / 2);
    pastelCache[name] = rgb;
    return rgb;
  }
  function pastelColour(name) {
    var rgb = pastelRGB(name);
    return rgb ? "rgb(" + rgb + ")" : "";
  }

  /* ---- Sticky banner stacking: each banner pins directly below the
   * header (or the banner above it) so several never overlap. ---- */
  function layoutBanners() {
    var header = document.querySelector(".site-header");
    var top = header ? header.offsetHeight : 54;
    document.querySelectorAll(".banner-banner").forEach(function (b) {
      b.style.top = top + "px";
      top += b.offsetHeight;
    });
  }
  layoutBanners();
  window.addEventListener("resize", layoutBanners);

  var charts = [];
  function rerenderCharts() {
    if (typeof Chart === "undefined") return;
    chartDefaults();
    charts.forEach(function (c) { if (c) c.destroy(); });
    charts = [];
    initCharts();
  }

  /* Parse a chart-data script node into {labels, datasets}. Returns null
   * when unusable so one bad node cannot abort the others. Tolerates
   * payloads that were double-encoded (a JSON string containing JSON),
   * as served by older builds. */
  function parsePayload(node) {
    var payload;
    try { payload = JSON.parse(node.textContent); } catch (e) { return null; }
    if (typeof payload === "string") {
      try { payload = JSON.parse(payload); } catch (e) { return null; }
    }
    if (!payload || typeof payload !== "object") return null;
    if (!Array.isArray(payload.labels)) payload.labels = [];
    if (!Array.isArray(payload.datasets)) payload.datasets = [];
    return payload;
  }

  /* ---- Currency formatting: symbol flows from the server settings via
   * <body data-currency-symbol> ---- */
  var SYMBOL = document.body.getAttribute("data-currency-symbol") || "£";
  function moneyTick(v) { return SYMBOL + abbrev(v); }
  function pctTick(v) { return (v * 100).toFixed(0) + "%"; }
  function fmtMoney(v) {
    return SYMBOL + Number(v).toLocaleString("en-GB", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  function fmtPct(v) { return (v * 100).toFixed(2) + "%"; }
  /* alpha-blends any CSS colour (hex or rgb()) into an rgba() string. */
  function hexAlpha(color, alpha) {
    var c = (color || "").trim();
    var m = c.match(/^#([0-9a-f]{6})$/i);
    if (m) {
      var n = parseInt(m[1], 16);
      return "rgba(" + ((n >> 16) & 255) + "," + ((n >> 8) & 255) + "," + (n & 255) + "," + alpha + ")";
    }
    m = c.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/i);
    if (m) return "rgba(" + m[1] + "," + m[2] + "," + m[3] + "," + alpha + ")";
    m = c.match(/^#([0-9a-f]{3})$/i);
    if (m) {
      var r = parseInt(m[1].charAt(0) + m[1].charAt(0), 16);
      var g = parseInt(m[1].charAt(1) + m[1].charAt(1), 16);
      var b = parseInt(m[1].charAt(2) + m[1].charAt(2), 16);
      return "rgba(" + r + "," + g + "," + b + "," + alpha + ")";
    }
    return c;
  }
  function abbrev(v) {
    var a = Math.abs(v);
    if (a >= 1e6) return (v / 1e6).toFixed(1) + "M";
    if (a >= 1e3) return (v / 1e3).toFixed(a >= 1e4 ? 0 : 1) + "k";
    return String(Math.round(v));
  }

  /* Whether slice i is toggled off (used for share-of-visible-total). */
  function chartHidden(chart, index) {
    return chart.getDataVisibility ? !chart.getDataVisibility(index) : false;
  }

  /* Scrollable HTML legend for big pies: one click row per slice, wired to
   * toggleDataVisibility so slices can be filtered on/off (the built-in
   * legend overflows past a few dozen entries). */
  function buildHtmlLegend(holderId, chart, labels, colors) {
    var holder = document.getElementById(holderId);
    if (!holder) return;
    holder.innerHTML = "";
    var data = chart.data.datasets[0].data || [];
    labels.forEach(function (label, i) {
      var row = document.createElement("button");
      row.type = "button";
      row.className = "hl-item";
      var chip = document.createElement("span");
      chip.className = "chip";
      chip.style.backgroundColor = colors[i] || "";
      row.appendChild(chip);
      var text = document.createElement("span");
      text.className = "hl-label";
      text.textContent = label;
      row.appendChild(text);
      var val = document.createElement("span");
      val.className = "hl-value";
      val.textContent = fmtMoney(Number(data[i]) || 0);
      row.appendChild(val);
      row.addEventListener("click", function () {
        chart.toggleDataVisibility(i);
        chart.update();
        row.classList.toggle("hl-off", !chart.getDataVisibility(i));
      });
      holder.appendChild(row);
    });
  }

  function initCharts() {
    if (typeof Chart === "undefined") return;
    chartDefaults();
    var nodes = document.querySelectorAll("script.chart-data");
    var cols = palette();
    nodes.forEach(function (node) {
      var canvas = document.getElementById(node.getAttribute("data-chart"));
      if (!canvas) return;
      var payload = parsePayload(node);
      if (!payload) return;
      var type = node.getAttribute("data-type") || "line";
      var isPercent = node.getAttribute("data-percent") === "1";
      var stacked = node.getAttribute("data-stacked") === "1";
      var filled = node.getAttribute("data-filled") === "1";
      var signColors = node.getAttribute("data-sign-color") === "1";
      // Plain-count charts (the return histogram): the y axis counts
      // months - no currency symbol, no percent.
      var plainCount = node.getAttribute("data-plain") === "1";
      // Level charts (value vs deposits): anchor the y axis at zero so a
      // flat series reads as its level, not as the bottom of the chart.
      var yZero = node.getAttribute("data-y-zero") === "1";
      // The allocation chart's stack preference is client-side: the stored
      // choice wins over the server-rendered default so switching never
      // reloads the page.
      if (canvas.id === "chart-alloc-time" && uiState()["allocation.stack"] === "lines") {
        stacked = false;
      }
      // Pastel name-colours are only for charts OF assets/classes (opt-in);
      // everything else keeps the standard CSS palette.
      var usePastel = node.getAttribute("data-pastel") === "1";
      var htmlLegendId = node.getAttribute("data-legend-target");
      var tooltipSingle = node.getAttribute("data-tooltip-single") === "1";

      function seriesColour(label, fallbackIndex) {
        if (usePastel && label) {
          var p = pastelColour(label);
          if (p) return p;
        }
        return cols[fallbackIndex % cols.length];
      }

      if (type === "pie" || type === "doughnut") {
        var sliceColors = payload.labels.map(function (label, i) { return seriesColour(label, i); });
        var pieLegend = { position: "right" };
        if (htmlLegendId) pieLegend = { display: false };
        var pieChart = new Chart(canvas, {
          type: type,
          data: {
            labels: payload.labels,
            datasets: [{
              data: payload.datasets[0].data,
              backgroundColor: sliceColors,
              borderWidth: 1, // hairline slice separation, not a fat white ring
            }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: {
              legend: pieLegend,
              tooltip: {
                callbacks: {
                  label: function (ctx) {
                    var data = ctx.dataset.data || [];
                    var total = 0;
                    for (var k = 0; k < data.length; k++) {
                      if (!chartHidden(ctx.chart, k)) {
                        var n = Number(data[k]);
                        if (!isNaN(n)) total += n;
                      }
                    }
                    var v = Number(ctx.parsed) || 0;
                    var share = total > 0 ? " (" + ((v / total) * 100).toFixed(1) + "%)" : "";
                    return ctx.label + ": " + fmtMoney(v) + share;
                  },
                },
              },
            },
          },
        });
        if (htmlLegendId) {
          buildHtmlLegend(htmlLegendId, pieChart, payload.labels, sliceColors);
        }
        charts.push(pieChart);
        return;
      }

      var datasets = payload.datasets.map(function (ds, i) {
        // Named series on asset/class charts keep their name-derived colour;
        // everything else cycles the CSS palette. ds.colorIndex pins a
        // series to a specific palette slot (track-record colour pairing).
        var color = ds.color || (ds.colorIndex !== undefined ? cols[ds.colorIndex % cols.length] : seriesColour(ds.label, i));
        var d = {
          label: ds.label,
          data: ds.data,
          backgroundColor: ds.bg || (type === "bar" ? color : "transparent"),
          borderColor: color,
          borderWidth: ds.width || 2,
          pointRadius: payload.labels.length > 80 ? 0 : 2,
          tension: 0.25,
          spanGaps: true,
        };
        if (ds.dashed) d.borderDash = [5, 4];
        if (filled || ds.filled) {
          d.fill = true;
          var rgb = usePastel ? pastelRGB(ds.label) : "";
          // Stacked bands need a solid fill to read as the series' own
          // colour (at 0.18 alpha the generator colour washes into the
          // card background - especially in dark mode, where the stacked
          // chart stopped matching the pie/legend chips entirely).
          var alpha = stacked ? 0.65 : 0.18;
          d.backgroundColor = rgb ? "rgba(" + rgb + "," + alpha + ")" : hexAlpha(color, alpha);
        }
        // Scenario band: fill down to the previous dataset (lower path).
        if (ds.band) {
          d.fill = "-1";
          d.borderWidth = 1;
          d.backgroundColor = hexAlpha(color, 0.13);
        }
        // Horizontal reference lines (projection targets): thin dashed grey.
        if (ds.constant) {
          d.borderColor = cssVar("--text-muted") || "#888";
          d.borderDash = [6, 4];
          d.borderWidth = 1;
          d.pointRadius = 0;
          d.backgroundColor = "transparent";
        }
        if (ds.stack !== undefined) d.stack = ds.stack;
        if (signColors && type === "bar") {
          var up = cssVar("--up") || "#2e9e5b";
          var dn = cssVar("--down") || "#cf3b49";
          var perBar = [];
          var series = ds.data || [];
          for (var k = 0; k < series.length; k++) {
            perBar.push(series[k] < 0 ? dn : up);
          }
          d.backgroundColor = perBar;
          d.borderColor = perBar;
        }
        return d;
      });

      // Stacked per-asset histories: hovering one segment should explain
      // that segment, not list every series on the x position.
      var tooltipFilter = null;
      if (tooltipSingle) {
        var lastMouseY = 0;
        canvas.addEventListener("mousemove", function (e) {
          var rect = canvas.getBoundingClientRect();
          lastMouseY = e.clientY - rect.top;
        });
        // Keep only the series whose stacked band contains the cursor.
        tooltipFilter = function (item) {
          var chart = item.chart;
          var idx = item.dataIndex;
          var meta = chart.getDatasetMeta(item.datasetIndex);
          var pt = meta.data[idx];
          if (!pt) return false;
          var prevY = chart.scales.y.getPixelForValue(0);
          for (var i = item.datasetIndex - 1; i >= 0; i--) {
            var m = chart.getDatasetMeta(i);
            if (!m.hidden && m.data && m.data[idx]) {
              prevY = m.data[idx].y;
              break;
            }
          }
          var lo = Math.min(prevY, pt.y) - 2;
          var hi = Math.max(prevY, pt.y) + 2;
          return lastMouseY >= lo && lastMouseY <= hi;
        };
      }

      var opts = {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: "index", intersect: false },
        scales: {
          x: { stacked: stacked, grid: { display: false }, ticks: { maxTicksLimit: 12 } },
          y: {
            stacked: stacked,
            beginAtZero: yZero,
            ticks: {
              callback: plainCount
                ? function (v) { return String(Math.round(v)); }
                : (isPercent ? pctTick : moneyTick),
            },
          },
        },
        plugins: {
          // A legend over a dozen-odd series (per-asset history) is noise.
          legend: {
            display: datasets.length > 1 && datasets.length <= 12,
            labels: { generateLabels: solidLegendLabels },
          },
          tooltip: {
            filter: tooltipFilter || undefined,
            callbacks: {
              label: function (ctx) {
                var v = ctx.parsed.y;
                if (plainCount) return ctx.dataset.label + ": " + Math.round(v) + " months";
                return ctx.dataset.label + ": " + (isPercent ? fmtPct(v) : fmtMoney(v));
              },
              // Solid tooltip swatches: match the legend keys by painting
              // the box with the series colour instead of the (usually
              // transparent) fill colour.
              labelColor: function (ctx) {
                var ds = ctx.dataset || {};
                var c = ds.borderColor;
                if (Array.isArray(c)) c = c[ctx.dataIndex];
                if (typeof c !== "string") c = ds.backgroundColor;
                if (Array.isArray(c)) c = c[ctx.dataIndex];
                if (typeof c !== "string") c = "rgba(128,128,128,.8)";
                return { backgroundColor: c, borderColor: c, borderWidth: 0 };
              },
            },
          },
        },
      };
      charts.push(new Chart(canvas, { type: type === "bar" ? "bar" : "line", data: { labels: payload.labels, datasets: datasets }, options: opts }));
    });

    /* Name-derived colour chips in tables ([data-pastel] spans). */
    document.querySelectorAll("[data-pastel]").forEach(function (el) {
      el.style.backgroundColor = pastelColour(el.getAttribute("data-pastel"));
    });
  }

  initCharts();

  /* ---- Allocation-over-time stack toggle: re-scales the chart in
   * place - no reload - and persists the choice ("stacked" default). ---- */
  function applyAllocationStack(mode) {
    var stackedOn = mode !== "lines";
    charts.forEach(function (c) {
      if (!c || !c.canvas || c.canvas.id !== "chart-alloc-time") return;
      c.options.scales.x.stacked = stackedOn;
      c.options.scales.y.stacked = stackedOn;
      c.update();
    });
    document.querySelectorAll("[data-alloc-stack] [data-stack-toggle]").forEach(function (b) {
      b.classList.toggle("active", b.getAttribute("data-stack-toggle") === (stackedOn ? "stacked" : "lines"));
    });
    uiSave("allocation.stack", stackedOn ? "" : "lines");
  }
  document.querySelectorAll("[data-alloc-stack] [data-stack-toggle]").forEach(function (b) {
    b.addEventListener("click", function () {
      applyAllocationStack(b.getAttribute("data-stack-toggle"));
    });
  });
  // Reflect a stored "lines" preference in the toggle highlight too (the
  // chart itself already picked it up during initCharts).
  (function initStackToggle() {
    var holder = document.querySelector("[data-alloc-stack]");
    if (!holder) return;
    var mode = uiState()["allocation.stack"] === "lines" ? "lines" : "stacked";
    holder.querySelectorAll("[data-stack-toggle]").forEach(function (b) {
      b.classList.toggle("active", b.getAttribute("data-stack-toggle") === mode);
    });
  })();

  /* ---- Sortable tables: th[data-sort] headers cycle asc/desc, reading
   * data-<key> attributes off each row. Numeric keys sort numerically,
   * name/class alphabetically; unknown values always sort last. ---- */
  document.querySelectorAll("table[data-sortable]").forEach(function (table) {
    var headers = Array.prototype.slice.call(table.querySelectorAll("th[data-sort]"));
    var tbody = table.querySelector("tbody");
    if (!tbody || !headers.length) return;
    var current = { key: headers[0].getAttribute("data-sort"), dir: 1 };
    var textual = function (key) { return key === "name" || key === "class"; };

    function cellValue(tr, key) {
      var raw = tr.getAttribute("data-" + key);
      if (textual(key)) return (raw || "").toLowerCase();
      var n = parseFloat(raw);
      return isNaN(n) ? null : n;
    }

    function sortRows() {
      var rows = Array.prototype.slice.call(tbody.querySelectorAll("tr"));
      rows.sort(function (a, b) {
        var av = cellValue(a, current.key);
        var bv = cellValue(b, current.key);
        if (av === null && bv === null) return 0;
        if (av === null) return 1;
        if (bv === null) return -1;
        var cmp;
        if (typeof av === "string") {
          cmp = av < bv ? -1 : av > bv ? 1 : 0;
        } else {
          cmp = av - bv === 0 ? 0 : av < bv ? -1 : 1;
        }
        return cmp * current.dir;
      });
      rows.forEach(function (r) { tbody.appendChild(r); });
    }

    headers.forEach(function (th) {
      th.classList.add("sortable");
      th.setAttribute("title", "Sort by " + th.textContent.trim());
      th.addEventListener("click", function () {
        var key = th.getAttribute("data-sort");
        if (current.key === key) {
          current.dir = -current.dir;
        } else {
          current.key = key;
          current.dir = 1;
        }
        headers.forEach(function (h) { h.classList.remove("asc", "desc"); });
        th.classList.add(current.dir === 1 ? "asc" : "desc");
        sortRows();
      });
    });
    headers[0].classList.add("asc"); // default: alphabetical by name (server order)
  });

  /* ---- Quick-update grid: column tab flow ---- */
  var grid = document.querySelector(".quick-grid");
  if (grid) {
    grid.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" && e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
      var cell = e.target;
      if (cell.tagName !== "INPUT") return;
      e.preventDefault();
      var col = cell.cellIndex !== undefined ? cell.cellIndex : 0;
      var row = cell.closest("tr");
      var next = e.key === "ArrowUp" ? row.previousElementSibling : row.nextElementSibling;
      if (!next) return;
      var target = next.querySelectorAll("input")[Array.prototype.indexOf.call(row.querySelectorAll("input"), cell)];
      if (target) target.focus();
    });
  }

  /* ---- Asset nav quick-filter (client-side, v1 pattern) ----
   * Every .asset-filter input filters the [data-asset-row] rows inside its
   * own card, so a page can carry several independent filter boxes. The
   * original #asset-filter (manage/breakdown open list) also persists its
   * text in localStorage, as before. */
  document.querySelectorAll(".asset-filter").forEach(function (filter) {
    var scope = filter.closest("section") || document;
    function apply() {
      var q = filter.value.toLowerCase();
      scope.querySelectorAll("[data-asset-row]").forEach(function (tr) {
        tr.style.display = tr.getAttribute("data-asset-row").toLowerCase().indexOf(q) >= 0 ? "" : "none";
      });
    }
    filter.addEventListener("input", apply);
    if (filter.id === "asset-filter") {
      var saved = localStorage.getItem("asset-filter");
      if (saved) { filter.value = saved; apply(); }
      filter.addEventListener("change", function () { localStorage.setItem("asset-filter", filter.value); });
    }
  });

  /* ---- Searchable dropdowns (select[data-picker]) ----
   * Progressive enhancement: the native select stays the source of truth
   * and works without JS; JS wraps it in a search box with a filtered,
   * grouped list. Options marked data-closed="1" get a "closed" pill. */
  function enhancePicker(sel) {
    if (sel.getAttribute("data-picker-done")) return;
    sel.setAttribute("data-picker-done", "1");

    var wrap = document.createElement("div");
    wrap.className = "picker";
    sel.parentNode.insertBefore(wrap, sel);
    sel.classList.add("picker-native");

    var toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "picker-toggle";
    var menu = document.createElement("div");
    menu.className = "picker-menu";
    menu.hidden = true;
    var search = document.createElement("input");
    search.type = "search";
    search.className = "picker-search";
    search.placeholder = "Search…";
    menu.appendChild(search);
    var list = document.createElement("div");
    list.className = "picker-list";
    menu.appendChild(list);

    function labelOf(option) {
      return option.textContent.replace(/\s*\(closed\)\s*$/, "").trim();
    }
    function renderItem(option) {
      var item = document.createElement("button");
      item.type = "button";
      item.className = "picker-item";
      var txt = document.createElement("span");
      txt.className = "picker-label";
      txt.textContent = labelOf(option);
      item.appendChild(txt);
      if (option.getAttribute("data-closed") === "1") {
        var pill = document.createElement("span");
        pill.className = "pill pill-closed";
        pill.textContent = "closed";
        item.appendChild(pill);
      }
      item.addEventListener("click", function () {
        sel.value = option.value;
        syncToggle();
        close();
        sel.dispatchEvent(new Event("change", { bubbles: true }));
      });
      return item;
    }
    function renderList() {
      list.innerHTML = "";
      var q = search.value.toLowerCase();
      var any = false;
      function addGroup(label, options) {
        var matches = options.filter(function (o) {
          return !q || labelOf(o).toLowerCase().indexOf(q) >= 0;
        });
        if (!matches.length) return;
        if (label) {
          var h = document.createElement("div");
          h.className = "picker-group";
          h.textContent = label;
          list.appendChild(h);
        }
        matches.forEach(function (o) { list.appendChild(renderItem(o)); any = true; });
      }
      var groups = sel.querySelectorAll("optgroup");
      if (groups.length) {
        Array.prototype.forEach.call(groups, function (g) {
          addGroup(g.label, Array.prototype.slice.call(g.querySelectorAll("option")));
        });
      } else {
        addGroup("", Array.prototype.slice.call(sel.querySelectorAll("option")));
      }
      if (!any) {
        var none = document.createElement("div");
        none.className = "picker-empty";
        none.textContent = "No matches";
        list.appendChild(none);
      }
    }
    function syncToggle() {
      var opt = sel.options[sel.selectedIndex];
      toggle.textContent = opt ? opt.textContent.trim() : "";
      toggle.classList.toggle("placeholder", !sel.value);
    }
    function open() {
      menu.hidden = false;
      search.value = "";
      renderList();
      search.focus();
    }
    function close() { menu.hidden = true; }
    toggle.addEventListener("click", function () {
      if (menu.hidden) open(); else close();
    });
    search.addEventListener("input", renderList);
    search.addEventListener("keydown", function (e) {
      if (e.key === "Escape") close();
      if (e.key === "Enter") {
        e.preventDefault();
        var first = list.querySelector(".picker-item");
        if (first) first.click();
      }
    });
    document.addEventListener("click", function (e) {
      if (!wrap.contains(e.target)) close();
    });
    syncToggle();
    wrap.appendChild(toggle);
    wrap.appendChild(menu);
    wrap.appendChild(sel);
  }
  document.querySelectorAll("select[data-picker]").forEach(enhancePicker);

  /* ---- Browse picker: page-dependent behaviour ----
   * Overview navigates to the chosen asset's/class's own page; the other
   * pages (breakdown/analytics/projections) stay put and scope themselves
   * via the /asset/{id}, /class/{id} path routes. Table links everywhere
   * still lead to the dedicated pages. */
  document.querySelectorAll("form.browse-picker").forEach(function (form) {
    form.addEventListener("change", function (e) {
      var sel = e.target;
      if (!sel || !sel.value) return;
      if (PAGE === "overview") {
        if (sel.name === "asset_id") {
          window.location.href = "/asset/" + encodeURIComponent(sel.value);
        } else if (sel.name === "class_id") {
          window.location.href = "/class/" + encodeURIComponent(sel.value);
        }
        return;
      }
      if (sel.name !== "asset_id" && sel.name !== "class_id") return;
      var kind = sel.name === "asset_id" ? "asset" : "class";
      var base = window.location.pathname.replace(/\/(asset|class)\/\d+(\/pay\/\d+)?$/, "");
      uiSave("scope." + sel.name, sel.value);
      uiSave("scope." + (kind === "asset" ? "class_id" : "asset_id"), "");
      window.location.href = base + "/" + kind + "/" + encodeURIComponent(sel.value);
    });
  });

  /* ---- Clear-selection link: drop the remembered scope so the bare page
   * doesn't immediately re-apply it, then let the navigation proceed. ---- */
  document.querySelectorAll("a.scope-clear").forEach(function (a) {
    a.addEventListener("click", function () {
      uiSave("scope.asset_id", "");
      uiSave("scope.class_id", "");
    });
  });

  /* ---- Flash banners: dismissible and auto-fading (the message rides a
   * one-shot cookie the server consumes while rendering - nothing to
   * strip from the URL). ---- */
  document.querySelectorAll(".banner-banner.flash").forEach(function (flash) {
    var close = flash.querySelector(".banner-close");
    if (close) close.addEventListener("click", function () { flash.remove(); layoutBanners(); });
    setTimeout(function () {
      flash.style.transition = "opacity .6s";
      flash.style.opacity = "0";
      setTimeout(function () { flash.remove(); layoutBanners(); }, 650);
    }, 5000);
  });

  /* ---- Server-round-trip controls (range presets, custom range,
   * projections horizon/history-window, ledger search): the choice is
   * saved to localStorage, mirrored into the UI cookie, and the page
   * reloads on its clean path - no query strings anywhere. ---- */
  document.querySelectorAll("[data-range-picker]").forEach(function (picker) {
    picker.querySelectorAll("[data-set-from]").forEach(function (b) {
      b.addEventListener("click", function () {
        uiSave("from", b.getAttribute("data-set-from") || "");
        uiSave("to", b.getAttribute("data-set-to") || "");
        window.location.reload();
      });
    });
    var custom = picker.querySelector("[data-range-custom]");
    if (custom) custom.addEventListener("submit", function (e) {
      e.preventDefault();
      var from = custom.querySelector('[name="from"]');
      var to = custom.querySelector('[name="to"]');
      if (!monthsOK([from, to])) return;
      uiSave("from", (from || {}).value || "");
      uiSave("to", (to || {}).value || "");
      window.location.reload();
    });
  });

  document.querySelectorAll("[data-proj-picker]").forEach(function (picker) {
    picker.querySelectorAll("[data-set-horizon]").forEach(function (b) {
      b.addEventListener("click", function () {
        uiSave("horizon", b.getAttribute("data-set-horizon") || "");
        window.location.reload();
      });
    });
    picker.querySelectorAll("[data-set-window]").forEach(function (b) {
      b.addEventListener("click", function () {
        uiSave("window", b.getAttribute("data-set-window") || "");
        window.location.reload();
      });
    });
  });

  document.querySelectorAll("[data-ledger-search]").forEach(function (form) {
    form.addEventListener("submit", function (e) {
      e.preventDefault();
      if (!monthsOK([form.querySelector('[name="from"]'), form.querySelector('[name="to"]')])) return;
      uiSave("q", (form.querySelector('[name="q"]') || {}).value || "");
      uiSave("from", (form.querySelector('[name="from"]') || {}).value || "");
      uiSave("to", (form.querySelector('[name="to"]') || {}).value || "");
      window.location.reload();
    });
    var clear = form.querySelector("[data-ledger-clear]");
    if (clear) clear.addEventListener("click", function () {
      uiSave("q", "");
      uiSave("from", "");
      uiSave("to", "");
      window.location.reload();
    });
  });

  /* ---- Collapsible sections remember their state ---- */
  document.querySelectorAll("details[data-store-open]").forEach(function (d) {
    var key = d.getAttribute("data-store-open");
    var stored = uiState()[key];
    if (stored !== undefined) d.open = !!stored;
    d.addEventListener("toggle", function () { uiSave(key, d.open); });
  });

  /* ---- Projection targets: removing a row POSTs the delete, then rebuilds
   * the list from GET /api/v1/settings - no confirm dialog, no page reload.
   * Falls back to the plain form round-trip when anything goes wrong. ---- */
  var targetsList = document.getElementById("targets-list");
  if (targetsList && !targetsList.querySelector("button[disabled]")) {
    var targetsCsrf = (targetsList.querySelector('input[name="csrf_token"]') || {}).value || "";
    var TARGET_DEL_SVG = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><path d="M10 11v6"/><path d="M14 11v6"/></svg>';
    function wireTargetDel(form) {
      form.addEventListener("submit", function (ev) {
        ev.preventDefault();
        var btn = form.querySelector("button");
        if (btn) btn.disabled = true;
        fetch(form.action, { method: "POST", body: new FormData(form) })
          .then(function (res) { if (!res.ok) throw new Error("remove failed"); })
          .then(function () {
            return fetch("/api/v1/settings", { headers: { "Accept": "application/json" } });
          })
          .then(function (res) { return res.json(); })
          .then(function (body) {
            var rows = (body && body.data) || [];
            var row = rows.find(function (s) { return s && s.key === "projection_targets"; });
            renderTargets(JSON.parse((row && row.value) || "[]"));
            // The form path delivers its confirmation as a flash cookie on
            // the next navigation; in-place there is no next navigation, so
            // clear it rather than let it surface later.
            document.cookie = "conspectus_flash=; Max-Age=0; path=/";
          })
          .catch(function () { form.submit(); });
      });
    }
    function renderTargets(list) {
      var table = targetsList.querySelector("table");
      if (!list.length) {
        if (table) table.remove();
        if (!targetsList.querySelector(".js-targets-empty")) {
          var p = document.createElement("p");
          p.className = "hint js-targets-empty";
          p.textContent = "No targets set.";
          targetsList.appendChild(p);
        }
        return;
      }
      if (!table) {
        table = document.createElement("table");
        table.className = "table";
        table.innerHTML = '<thead><tr><th>Target</th><th class="row-actions-head">Actions</th></tr></thead><tbody></tbody>';
        var empty = targetsList.querySelector(".js-targets-empty");
        if (empty) empty.remove();
        targetsList.insertBefore(table, targetsList.firstChild);
      }
      var tbody = table.tBodies[0];
      tbody.textContent = "";
      list.forEach(function (v) {
        var tr = document.createElement("tr");
        tr.innerHTML =
          "<td>" + fmtMoney(v) + "</td>" +
          '<td class="row-actions"><form method="post" action="/projections/targets/delete" class="inline js-target-del">' +
          '<input type="hidden" name="csrf_token" value="' + targetsCsrf + '">' +
          '<input type="hidden" name="value" value="' + v + '">' +
          '<button class="btn-sm danger" type="submit" title="Remove target">' + TARGET_DEL_SVG + "Remove</button>" +
          "</form></td>";
        tbody.appendChild(tr);
        wireTargetDel(tr.querySelector("form"));
      });
    }
    targetsList.querySelectorAll(".js-target-del").forEach(wireTargetDel);
  }

  /* ---- Quick-update grid: skipping a row greys it out and disables its
   * inputs so what will actually be written stays obvious. The header's
   * "skip all" master toggle flips every row at once - tick it, then
   * untick the one asset you want to update. ---- */
  var skipBoxes = Array.prototype.slice.call(document.querySelectorAll(".quick-grid input[type=checkbox][name^=skip_]"));
  skipBoxes.forEach(function (cb) {
    var row = cb.closest("tr");
    if (!row) return;
    function apply() {
      row.classList.toggle("row-skipped", cb.checked);
      row.querySelectorAll("input[type=text]").forEach(function (i) { i.disabled = cb.checked; });
    }
    cb.addEventListener("change", apply);
    apply();
  });
  document.querySelectorAll(".quick-grid [data-skip-all]").forEach(function (master) {
    master.addEventListener("change", function () {
      skipBoxes.forEach(function (cb) {
        if (cb.checked !== master.checked) {
          cb.checked = master.checked;
          cb.dispatchEvent(new Event("change"));
        }
      });
    });
  });

  /* ---- Fetch pagination: when a pager declares a JSON endpoint, Next/
   * Prev fetch that page in the background and swap the table body - no
   * full page reload. The server-rendered links remain the no-JS path. ---- */
  function escHtml(s) {
    var d = document.createElement("div");
    d.textContent = s == null ? "" : String(s);
    return d.innerHTML;
  }
  function pagerRowHTML(kind, r, csrf, readonly, returnUrl) {
    var dis = readonly ? " disabled" : "";
    if (kind === "log") {
      var fid = "log-edit-" + r.id;
      return "<tr>" +
        '<td><form id="' + fid + '" method="post" action="/manage/log/update" class="inline">' +
        '<input type="hidden" name="csrf_token" value="' + csrf + '">' +
        '<input type="hidden" name="id" value="' + r.id + '">' +
        '<input type="hidden" name="return" value="' + escHtml(returnUrl) + '"></form>' +
        '<input type="date" name="date" value="' + escHtml(r.date) + '" form="' + fid + '"' + dis + "></td>" +
        "<td>" + escHtml(r.asset) + "</td>" +
        '<td class="num"><input type="text" inputmode="decimal" name="deposit" value="' + escHtml(r.deposit) + '" form="' + fid + '" class="num-input"' + dis + "></td>" +
        '<td class="num"><input type="text" inputmode="decimal" name="value" value="' + escHtml(r.value) + '" form="' + fid + '" class="num-input"' + dis + "></td>" +
        '<td class="row-actions"><button class="btn-sm" type="submit" form="' + fid + '" title="Save this entry"' + dis + ">Save</button>" +
        '<form method="post" action="/manage/log/delete" class="inline" onsubmit="return confirm(\'Delete this log entry? This cannot be undone.\');">' +
        '<input type="hidden" name="csrf_token" value="' + csrf + '">' +
        '<input type="hidden" name="id" value="' + r.id + '">' +
        '<input type="hidden" name="return" value="' + escHtml(returnUrl) + '">' +
        '<button class="btn-sm danger" type="submit"' + dis + ">Delete</button></form></td></tr>";
    }
    if (kind === "payment") {
      var pid = "pay-edit-" + r.id;
      return "<tr>" +
        '<td><form id="' + pid + '" method="post" action="/manage/payment/update" class="inline">' +
        '<input type="hidden" name="csrf_token" value="' + csrf + '">' +
        '<input type="hidden" name="id" value="' + r.id + '">' +
        '<input type="hidden" name="return" value="' + escHtml(returnUrl) + '"></form>' +
        '<input type="date" name="date" value="' + escHtml(r.date) + '" form="' + pid + '"' + dis + "></td>" +
        "<td>" + escHtml(r.asset) + "</td>" +
        '<td class="num"><input type="text" inputmode="decimal" name="amount" value="' + escHtml(r.amount) + '" form="' + pid + '" class="num-input"' + dis + "></td>" +
        '<td class="row-actions"><button class="btn-sm" type="submit" form="' + pid + '" title="Save this payment"' + dis + ">Save</button>" +
        '<form method="post" action="/manage/payment/delete" class="inline" onsubmit="return confirm(\'Delete this payment? This cannot be undone.\');">' +
        '<input type="hidden" name="csrf_token" value="' + csrf + '">' +
        '<input type="hidden" name="id" value="' + r.id + '">' +
        '<input type="hidden" name="return" value="' + escHtml(returnUrl) + '">' +
        '<button class="btn-sm danger" type="submit"' + dis + ">Delete</button></form></td></tr>";
    }
    // Read-only lists (asset/class pages): date, optional asset, amount.
    var cells = "<td>" + escHtml(r.date) + "</td>";
    if (kind === "pay-class") cells += "<td>" + escHtml(r.asset) + "</td>";
    return "<tr>" + cells + '<td class="num">' + fmtMoney(Number(r.amount) || 0) + "</td></tr>";
  }
  document.querySelectorAll("nav.pagination[data-endpoint]").forEach(function (nav) {
    var target = document.getElementById(nav.getAttribute("data-target"));
    if (!target) return;
    var endpoint = nav.getAttribute("data-endpoint");
    var kind = nav.getAttribute("data-kind");
    var urlTpl = nav.getAttribute("data-url-tpl") || "";
    // The CSRF token for JS-built edit/delete forms comes from any form on
    // the page (the partial can't reach the root template data).
    var tokenInput = document.querySelector('input[name="csrf_token"]');
    var csrf = tokenInput ? tokenInput.value : "";
    var readonly = document.body.getAttribute("data-readonly") === "1";
    var totalPages = parseInt(nav.getAttribute("data-totalpages") || "1", 10);
    var cur = parseInt(nav.getAttribute("data-page") || "1", 10);
    var perPage = nav.getAttribute("data-perpage") || "20";
    var returnUrl = window.location.pathname;
    var loading = false;

    function buildUrl(page) {
      return endpoint + "/p/" + page + "/per/" + encodeURIComponent(perPage || "20");
    }
    function pageHref(n) {
      if (!urlTpl) return window.location.pathname;
      // The server renders the placeholder as PAGE ({p} would be
      // percent-encoded by the template's attribute escaper).
      return urlTpl.indexOf("PAGE") >= 0
        ? urlTpl.replace("PAGE", String(n))
        : decodeURIComponent(urlTpl).replace("{p}", String(n));
    }
    function refreshLinks() {
      nav.classList.toggle("no-prev", cur <= 1);
      nav.classList.toggle("no-next", cur >= totalPages);
      var info = nav.querySelector(".page-info");
      if (info) {
        var total = nav.getAttribute("data-total");
        info.textContent = "Page " + cur + " of " + totalPages + (total ? " · " + total + " rows" : "");
      }
      var prev = nav.querySelector("a.pager-prev");
      if (prev) { prev.setAttribute("data-page", String(cur - 1)); prev.setAttribute("href", pageHref(cur - 1)); }
      var next = nav.querySelector("a.pager-next");
      if (next) { next.setAttribute("data-page", String(cur + 1)); next.setAttribute("href", pageHref(cur + 1)); }
      var holder = nav.querySelector(".pager-pages-wrap");
      if (holder) {
        var html = "";
        var start = Math.max(1, cur - 2), end = Math.min(totalPages, cur + 2);
        if (start > 1) html += '<a class="pager-num" data-page="1" href="' + pageHref(1) + '">1</a><span class="dots">…</span>';
        for (var i = start; i <= end; i++) {
          html += '<a class="pager-num' + (i === cur ? " current" : "") + '" data-page="' + i + '" href="' + pageHref(i) + '">' + i + "</a>";
        }
        if (end < totalPages) {
          html += '<span class="dots">…</span><a class="pager-num" data-page="' + totalPages + '" href="' + pageHref(totalPages) + '">' + totalPages + "</a>";
        }
        holder.innerHTML = html;
      }
    }
    function load(page) {
      if (loading || page < 1 || (page > totalPages && totalPages > 0)) return;
      loading = true;
      fetch(buildUrl(page), { headers: { "Accept": "application/json" } })
        .then(function (res) { return res.json(); })
        .then(function (j) {
          var d = j && j.data;
          if (!d || !Array.isArray(d.rows)) return;
          cur = d.page;
          totalPages = d.pages;
          nav.setAttribute("data-totalpages", totalPages);
          nav.setAttribute("data-total", d.total);
          nav.setAttribute("data-page", cur);
          var html = "";
          d.rows.forEach(function (r) { html += pagerRowHTML(kind, r, csrf, readonly, returnUrl); });
          target.innerHTML = html;
          // Address bar follows the page number via the path template -
          // no query string, and a refresh lands on the same page.
          var path = pageHref(cur);
          if (path) window.history.replaceState(null, "", path);
          returnUrl = path || window.location.pathname;
          refreshLinks();
        })
        .catch(function () {})
        .then(function () { loading = false; });
    }
    nav.addEventListener("click", function (e) {
      var a = e.target.closest("a");
      if (!a) return;
      e.preventDefault();
      if (a.classList.contains("pager-prev")) load(cur - 1);
      else if (a.classList.contains("pager-next")) load(cur + 1);
      else if (a.getAttribute("data-page")) load(parseInt(a.getAttribute("data-page"), 10));
    });
    var ppSel = nav.querySelector("select[data-perpage-select]");
    if (ppSel) ppSel.addEventListener("change", function () {
      perPage = ppSel.value;
      uiSave("per_page", perPage === "20" ? "" : perPage);
      load(1);
    });
    refreshLinks();
  });

  /* ---- Fit page-head titles ----
   * Long asset/class names scale their h1 down to the space left of the
   * range picker, so the picker sits in the same place on every page.
   * CSS (nowrap + ellipsis) is the floor; chips and meta are rem-sized
   * and stay constant while the name text shrinks, hence the few
   * convergence passes. */
  var TITLE_MIN_PX = 13;
  function fitPageHeads() {
    document.querySelectorAll(".page-head h1").forEach(function (h) {
      h.style.fontSize = "";
      var size = parseFloat(getComputedStyle(h).fontSize);
      for (var i = 0; i < 6; i++) {
        /* 1px slack: sub-pixel overflow still draws the ellipsis mark,
           which would clip the tail of the heading inline content. */
        var avail = h.clientWidth - 1, need = h.scrollWidth;
        if (need <= avail) break;
        var next = Math.max(TITLE_MIN_PX, size * avail / need);
        if (next >= size) break;
        h.style.fontSize = next.toFixed(2) + "px";
        if (next <= TITLE_MIN_PX) break;
        size = next;
      }
    });
  }
  fitPageHeads();
  window.addEventListener("resize", fitPageHeads);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(fitPageHeads);
})();
