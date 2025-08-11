// DOM Elements - Keep these as constants
const statusEl = document.getElementById("status");
const filterPanel = document.getElementById("filter-panel");
const filterContent = document.getElementById("filter-content");
const resizer = document.getElementById("resizer");
const logPanel = document.getElementById("log-panel");
const searchInput = document.getElementById("search-input");
const searchClear = document.getElementById("search-clear");
const regexpCheckbox = document.getElementById("regexp-checkbox");
const filterSearchInput = document.getElementById("filter-search-input");
const filterSearchClear = document.getElementById("filter-search-clear");
const resetFiltersBtn = document.getElementById("reset-filters");

// Application State Manager
class AppStateManager {
  constructor() {
    // Application state
    this.state = {
      // Search & Filter state (persisted in URL)
      filters: {},
      searchTerm: "",
      regexpEnabled: false,
      filterSearchTerm: "",
      stickyFilters: new Set(),

      // Runtime state (not persisted)
      index: {},
      logs: [],
      serverStatus: null,
    };

    // Connection state
    this.connection = {
      ws: null,
      wsOpen: false,
      domContentLoaded: false,
      reconnectInterval: null,
    };

    // UI state
    this.ui = {
      isResizing: false,
    };
  }

  // State getters
  getFilters() {
    return this.state.filters;
  }
  getSearchTerm() {
    return this.state.searchTerm;
  }
  getRegexpEnabled() {
    return this.state.regexpEnabled;
  }
  getFilterSearchTerm() {
    return this.state.filterSearchTerm;
  }
  getStickyFilters() {
    return this.state.stickyFilters;
  }
  getIndex() {
    return this.state.index;
  }
  getLogs() {
    return this.state.logs;
  }
  getServerStatus() {
    return this.state.serverStatus;
  }

  // State setters with URL persistence
  setFilters(filters) {
    this.state.filters = filters;
    this.updateURL();
  }

  setSearchTerm(searchTerm) {
    this.state.searchTerm = searchTerm;
    this.updateURL();
  }

  setRegexpEnabled(enabled) {
    this.state.regexpEnabled = enabled;
    this.updateURL();
  }

  setFilterSearchTerm(term) {
    this.state.filterSearchTerm = term;
    this.updateURL();
  }

  setStickyFilters(stickyFilters) {
    this.state.stickyFilters = stickyFilters;
    this.updateURL();
  }

  // Runtime state setters (no URL persistence)
  setIndex(index) {
    this.state.index = index;
  }

  setLogs(logs) {
    this.state.logs = logs;
  }

  setServerStatus(status) {
    this.state.serverStatus = status;
  }

  // Batch update for filters
  updateFilters(key, value, add = true) {
    if (!this.state.filters[key]) this.state.filters[key] = [];

    const idx = this.state.filters[key].indexOf(value);
    if (add && idx === -1) {
      this.state.filters[key].push(value);
    } else if (!add && idx !== -1) {
      this.state.filters[key].splice(idx, 1);
    }

    if (this.state.filters[key].length === 0) {
      delete this.state.filters[key];
    }

    this.updateURL();
  }

  toggleStickyFilter(key) {
    if (this.state.stickyFilters.has(key)) {
      this.state.stickyFilters.delete(key);
    } else {
      this.state.stickyFilters.add(key);
    }
    this.updateURL();
  }

  resetFilters() {
    this.state.filters = {};
    this.updateURL();
  }

  // URL state management
  updateURL() {
    const params = new URLSearchParams();

    // Add filters
    if (Object.keys(this.state.filters).length > 0) {
      params.set("filters", JSON.stringify(this.state.filters));
    }

    // Add search term
    if (this.state.searchTerm.trim()) {
      params.set("search", this.state.searchTerm.trim());
    }

    // Add regexp flag
    if (this.state.regexpEnabled) {
      params.set("regexp", "true");
    }

    // Add filter search term
    if (this.state.filterSearchTerm.trim()) {
      params.set("filterSearch", this.state.filterSearchTerm.trim());
    }

    // Add sticky filters
    if (this.state.stickyFilters.size > 0) {
      params.set("sticky", Array.from(this.state.stickyFilters).join(","));
    }

    const actualUrl = `${window.location.pathname}${window.location.search}`;
    const newURL = `${window.location.pathname}${params.toString() ? "?" + params.toString() : ""}`;
    if (newURL !== actualUrl) {
      window.history.pushState({}, "", newURL);
    }
  }

  loadStateFromURL() {
    const params = new URLSearchParams(window.location.search);

    // Load filters
    const filtersParam = params.get("filters");
    if (filtersParam) {
      try {
        const parsedFilters = JSON.parse(filtersParam);
        if (typeof parsedFilters === "object" && parsedFilters !== null) {
          this.state.filters = parsedFilters;
        }
      } catch (e) {
        console.warn("Invalid filters in URL:", e);
        this.state.filters = {};
      }
    } else {
      this.state.filters = {};
    }

    // Load search term
    const searchParam = params.get("search");
    this.state.searchTerm = searchParam || "";
    if (searchInput) {
      searchInput.value = this.state.searchTerm;
    }
    updateClearButton(this.state.searchTerm, searchClear);

    // Load regexp flag
    const regexpParam = params.get("regexp");
    this.state.regexpEnabled = regexpParam === "true";
    if (regexpCheckbox) {
      regexpCheckbox.checked = this.state.regexpEnabled;
    }

    // Load filter search term
    const filterSearchParam = params.get("filterSearch");
    this.state.filterSearchTerm = filterSearchParam || "";
    if (filterSearchInput) {
      filterSearchInput.value = this.state.filterSearchTerm;
    }
    updateClearButton(this.state.filterSearchTerm, filterSearchClear);

    // Load sticky filters
    const stickyParam = params.get("sticky");
    if (stickyParam) {
      this.state.stickyFilters = new Set(
        stickyParam.split(",").filter((s) => s.trim()),
      );
    } else {
      this.state.stickyFilters = new Set();
    }
  }

  // Connection management
  getWebSocket() {
    return this.connection.ws;
  }
  setWebSocket(ws) {
    this.connection.ws = ws;
  }

  isWebSocketOpen() {
    return this.connection.wsOpen;
  }
  setWebSocketOpen(open) {
    this.connection.wsOpen = open;
  }

  isDOMContentLoaded() {
    return this.connection.domContentLoaded;
  }
  setDOMContentLoaded(loaded) {
    this.connection.domContentLoaded = loaded;
  }

  getReconnectInterval() {
    return this.connection.reconnectInterval;
  }
  setReconnectInterval(interval) {
    this.connection.reconnectInterval = interval;
  }

  // UI state management
  isResizing() {
    return this.ui.isResizing;
  }
  setResizing(resizing) {
    this.ui.isResizing = resizing;
  }
}

// Initialize state manager
const appState = new AppStateManager();

// WebSocket connection management
function connectWS() {
  const ws = new WebSocket(`ws://${location.host}/ws`);
  appState.setWebSocket(ws);

  ws.onopen = () => {
    onWebsocketOpen();
    const reconnectInterval = appState.getReconnectInterval();
    if (reconnectInterval) {
      clearInterval(reconnectInterval);
      appState.setReconnectInterval(null);
    }
  };

  ws.onclose = () => {
    renderStatus("Reconnecting...", "orange");
    if (!appState.getReconnectInterval()) {
      const interval = setInterval(() => {
        renderStatus("Reconnecting...", "orange");
        connectWS();
      }, 1000);
      appState.setReconnectInterval(interval);
    }
  };

  ws.onerror = () => renderStatus("Error", "red");
  ws.onmessage = (e) => handleWSMessage(JSON.parse(e.data));
}

function handleWSMessage(msg) {
  switch (msg.type) {
    case "update_index":
      Object.assign(appState.getIndex(), msg.payload);
      renderFilters();
      break;
    case "drop_index":
      const index = appState.getIndex();
      for (const k of msg.payload) delete index[k];
      renderFilters();
      break;
    case "set_logs":
      appState.setLogs(msg.payload.logs);
      renderLogs();
      appState.setIndex(msg.payload.indexCounts);
      renderFilters();
      renderStatus();
      break;
    case "add_logs":
      const logs = appState.getLogs();
      logs.push(...msg.payload);
      if (logs.length > 1000) {
        appState.setLogs(logs.slice(-1000));
      }
      renderLogs();
      renderStatus();
      break;
    case "set_status":
      appState.setServerStatus(msg.payload);
      renderStatus();
      break;
  }
}

function sendSearchRequest() {
  const payload = { filters: { ...appState.getFilters() } };
  const searchTerm = appState.getSearchTerm();

  if (searchTerm.trim()) {
    payload.searchTerm = searchTerm.trim();
  }
  payload.regexp = appState.getRegexpEnabled();

  const ws = appState.getWebSocket();
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "set_search", payload: payload }));
  } else {
    console.debug("websocket not readyState");
  }
}

function renderStatus(text, color) {
  text ||= "Connected";
  color ||= "green";
  let statusText = text;
  const serverStatus = appState.getServerStatus();
  const logs = appState.getLogs();

  if (serverStatus && text === "Connected") {
    const memoryMB = Math.round(serverStatus.allocatedMemory / 1024 / 1024);
    statusText = `Connected | ${logs.length}/${serverStatus.logsStored} logs | ${memoryMB} MB`;
  }
  statusEl.textContent = statusText;
  statusEl.style.color = color;
}

function renderFilters() {
  filterContent.innerHTML = "";

  const index = appState.getIndex();
  const filterSearchTerm = appState.getFilterSearchTerm();
  const stickyFilters = appState.getStickyFilters();
  const filters = appState.getFilters();

  // Filter the index keys based on filterSearchTerm
  const filteredKeys = Object.keys(index).filter((key) => {
    if (!filterSearchTerm.trim()) return true;
    return key.toLowerCase().includes(filterSearchTerm.toLowerCase());
  });

  // Sort filtered keys to prioritize sticky filters
  filteredKeys.sort((a, b) => {
    const aIsSticky = stickyFilters.has(a);
    const bIsSticky = stickyFilters.has(b);

    if (aIsSticky && !bIsSticky) return -1;
    if (!aIsSticky && bIsSticky) return 1;
    return a.localeCompare(b);
  });

  for (const key of filteredKeys) {
    const box = document.createElement("div");
    box.className = "filter-box";
    if (stickyFilters.has(key)) {
      box.classList.add("sticky");
    }

    const title = document.createElement("div");
    title.className = "filter-title";

    const titleText = document.createElement("span");
    titleText.textContent = key;
    title.appendChild(titleText);

    const stickyToggle = document.createElement("button");
    stickyToggle.className = "sticky-toggle";
    stickyToggle.innerHTML = "📌";
    stickyToggle.title = "Toggle sticky";
    if (stickyFilters.has(key)) {
      stickyToggle.classList.add("active");
    }
    stickyToggle.onclick = function (e) {
      e.stopPropagation();
      toggleSticky(key);
    };
    title.appendChild(stickyToggle);

    box.appendChild(title);
    const values = index[key];
    for (const val in values) {
      const btn = document.createElement("button");
      btn.textContent = `${val} (${values[val]})`;
      btn.dataset["testid"] = `filter-btn:${key}:${val}`;
      btn.className = "filter-btn";
      if (values[val] == 0) {
        btn.className = "filter-btn filter-zero-matches";
      }
      if (filters[key] && filters[key].includes(val))
        btn.classList.add("active");
      btn.onclick = function () {
        toggleFilter(this, key, val);
      };
      box.appendChild(btn);
    }
    filterContent.appendChild(box);
  }
}

function toggleFilter(btn, key, val) {
  const isActive = btn.classList.contains("active");
  if (!isActive) {
    btn.classList.add("active");
    appState.updateFilters(key, val, true);
  } else {
    btn.classList.remove("active");
    appState.updateFilters(key, val, false);
  }
  sendSearchRequest();
}

function toggleSticky(key) {
  appState.toggleStickyFilter(key);
  renderFilters();
}

function setupSearch() {
  let searchTimeout;
  searchInput.addEventListener("input", (e) => {
    appState.setSearchTerm(e.target.value);
    updateClearButton(appState.getSearchTerm(), searchClear);
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      sendSearchRequest();
    }, 300); // Debounce search requests
  });

  // Regexp checkbox functionality
  regexpCheckbox.addEventListener("change", (e) => {
    appState.setRegexpEnabled(e.target.checked);
    sendSearchRequest();
  });

  // Clear button functionality
  setupClearButton(searchInput, searchClear, () => {
    appState.setSearchTerm("");
    appState.setRegexpEnabled(false);
    regexpCheckbox.checked = false;
    sendSearchRequest();
  });
}

function resetAllFilters() {
  appState.resetFilters();
  sendSearchRequest();
  renderFilters();
}

function setupFilterSearch() {
  filterSearchInput.addEventListener("input", (e) => {
    appState.setFilterSearchTerm(e.target.value);
    updateClearButton(appState.getFilterSearchTerm(), filterSearchClear);
    renderFilters();
  });

  // Clear button functionality for filter search
  setupClearButton(filterSearchInput, filterSearchClear, () => {
    appState.setFilterSearchTerm("");
    renderFilters();
  });
}

function setupFullscreen() {
  const fullscreenToggle = document.getElementById("fullscreen-toggle");
  fullscreenToggle.addEventListener("click", () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().catch((err) => {
        console.log(`Error attempting to enable fullscreen: ${err.message}`);
      });
    } else {
      document.exitFullscreen();
    }
  });
}

function renderLogs() {
  const logs = appState.getLogs();
  const searchTerm = appState.getSearchTerm();

  // Check if user is at the bottom before rendering
  const isAtBottom =
    logPanel.scrollTop + logPanel.clientHeight >= logPanel.scrollHeight - 10;

  logPanel.innerHTML = "";

  logs.forEach((log) => {
    const entry = document.createElement("div");
    entry.className = "log-entry";

    // Determine log level and add appropriate class
    const level = getLogLevel(log);
    if (level) {
      entry.classList.add(level.toLowerCase());
    }

    // Format timestamp if present
    let timestamp = "";
    if (log.timestamp || log.datetime) {
      const timeValue = log.timestamp || log.datetime;
      timestamp = `<div class="timestamp">${new Date(timeValue).toISOString()}</div>`;
    }

    // Format level if present
    let levelDisplay = "";
    if (log.level || log.level_name) {
      const levelValue = log.level_name || log.level;
      levelDisplay = `<span class="level">${levelValue}</span>`;
    }

    // Format message if present
    let message = "";
    if (log.message) {
      message = `<div class="message">${escapeHtmlAndHighlightSearchTerm(log.message, searchTerm)}</div>`;
    }

    // Format other properties
    const propertyKeys = Object.keys(log).filter(
      (key) =>
        !["timestamp", "datetime", "level", "level_name", "message"].includes(
          key,
        ),
    );

    let propertiesDisplay = "";
    if (propertyKeys.length > 0) {
      const propertyRows = propertyKeys
        .map((key) => {
          const value = log[key];
          const valueClass = getValueClass(value);
          const formattedValue = formatValue(value, searchTerm);
          return `<tr>
            <th class="property-name ${valueClass}">${escapeHtml(key)}</th>
            <td class="property-value ${valueClass}">${formattedValue}</td>
          </tr>`;
        })
        .join("");

      propertiesDisplay = `<table class="properties-table">
        <colgroup>
          <col style="width: 1px;">
        </colgroup>
        <tbody>${propertyRows}</tbody>
      </table>`;
    }

    entry.innerHTML = `
      ${timestamp}
      ${levelDisplay}
      ${message}
      ${propertiesDisplay}
    `;

    logPanel.appendChild(entry);
  });

  // Auto-scroll to bottom if user was at bottom before
  if (isAtBottom) {
    logPanel.scrollTop = logPanel.scrollHeight;
  }
}

function getLogLevel(log) {
  const level = (log.level_name || log.level || "").toString().toLowerCase();
  if (["error", "critical"].includes(level)) return "error";
  if (["warn", "warning"].includes(level)) return "warn";
  if (["info", "information"].includes(level)) return "info";
  if (["debug"].includes(level)) return "debug";
  return null;
}

function getValueClass(value) {
  if (typeof value === "string") return "string";
  if (typeof value === "number") return "number";
  if (typeof value === "boolean") return "boolean";
  if (typeof value === "object") return "object";
  if (value === null || value === undefined) return "null";
  return "";
}

function formatValue(value, searchTerm) {
  if (value === null || value === undefined) return "null";
  if (typeof value === "object") {
    return syntaxHighlightJson(value, searchTerm);
  }

  let stringValue = value.toString();
  return escapeHtmlAndHighlightSearchTerm(stringValue, searchTerm);
}

function escapeHtmlAndHighlightSearchTerm(stringValue, searchTerm) {
  stringValue = highlightSearchTerm(stringValue, searchTerm);

  // Escape HTML but preserve our highlight spans
  return stringValue
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/&lt;span class="highlight"&gt;/g, '<span class="highlight">')
    .replace(/&lt;\/span&gt;/g, "</span>");
}

function syntaxHighlightJson(json, searchTerm) {
  json = JSON.stringify(json, null, 4); // Pretty-print with 4-space indent

  json = json
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  return json.replace(
    /("(\\u[\da-fA-F]{4}|\\[^u]|[^\\"])*"(?:\s*:)?|\b(true|false|null)\b|\b\d+\.?\d*\b)/g,
    (match) => {
      let className = "number";
      if (/^"/.test(match)) {
        className = /:$/.test(match) ? "key" : "string";
      } else if (/true|false/.test(match)) {
        className = "boolean";
      } else if (/null/.test(match)) {
        className = "null";
      }
      return `<span class="${className}">${highlightSearchTerm(match, searchTerm)}</span>`;
    },
  );
}

function highlightSearchTerm(stringValue, searchTerm) {
  if (searchTerm && searchTerm.trim()) {
    const searchRegex = new RegExp(`(${escapeRegex(searchTerm.trim())})`, "gi");
    stringValue = stringValue.replace(
      searchRegex,
      '<span class="highlight">$1</span>',
    );
  }
  return stringValue;
}

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function setupResetFilters() {
  resetFiltersBtn.addEventListener("click", resetAllFilters);
}

function setupResizer() {
  resizer.addEventListener("mousedown", (e) => {
    appState.setResizing(true);
    e.preventDefault();
  });

  document.addEventListener("mousemove", (e) => {
    if (!appState.isResizing()) return;

    const newWidth = e.clientX - filterPanel.offsetLeft;
    const minWidth = 228;
    const maxWidth = 600;

    if (newWidth >= minWidth && newWidth <= maxWidth) {
      filterPanel.style.width = newWidth + "px";
    }
  });

  document.addEventListener("mouseup", () => {
    appState.setResizing(false);
  });
}

function setupExpandJson() {
  document.addEventListener("click", (e) => {
    const isPropertyName = e.target.classList.contains("property-name");
    const hasObjectValue = e.target.classList.contains("object");
    if (isPropertyName && hasObjectValue) {
      let style = e.target.nextElementSibling.style;
      if (style.whiteSpace === "break-spaces") {
        style.whiteSpace = "normal";
      } else {
        style.whiteSpace = "break-spaces";
      }
    }
  });
}

// Generic function to update clear button visibility
function updateClearButton(inputValue, clearButton) {
  if (inputValue.trim()) {
    clearButton.classList.add("visible");
  } else {
    clearButton.classList.remove("visible");
  }
}

// Generic function to setup clear button functionality
function setupClearButton(inputElement, clearButton, onClear) {
  clearButton.addEventListener("click", () => {
    inputElement.value = "";
    updateClearButton("", clearButton);
    if (onClear) onClear();
  });
}

// Handle browser navigation
window.addEventListener("popstate", () => {
  appState.loadStateFromURL();
  renderFilters();
  sendSearchRequest();
});

// Initialize
document.addEventListener("DOMContentLoaded", () => {
  appState.setDOMContentLoaded(true);
  if (appState.isWebSocketOpen()) {
    initApplication();
  }
});

function onWebsocketOpen() {
  appState.setWebSocketOpen(true);
  if (appState.isDOMContentLoaded()) {
    initApplication();
  }
  renderStatus();
  sendSearchRequest();
}

function initApplication() {
  appState.loadStateFromURL();
  renderStatus();
  sendSearchRequest();
}

// Initialize everything
connectWS();
setupSearch();
setupFilterSearch();
setupResetFilters();
setupFullscreen();
setupResizer();
setupExpandJson();
updateClearButton(appState.getSearchTerm(), searchClear);
updateClearButton(appState.getFilterSearchTerm(), filterSearchClear);
