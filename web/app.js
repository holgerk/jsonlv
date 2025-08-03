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

let ws;
let wsOpen = false;
let domContentLoaded = false;
let reconnectInterval = null;
let filters = {};
let searchTerm = "";
let regexpEnabled = false;
let filterSearchTerm = "";
let index = {};
let logs = [];
let serverStatus = null;
let stickyFilters = new Set();

let isResizing = false;

// URL state management
function updateURL() {
  const params = new URLSearchParams();

  // Add filters
  if (Object.keys(filters).length > 0) {
    params.set("filters", JSON.stringify(filters));
  }

  // Add search term
  if (searchTerm.trim()) {
    params.set("search", searchTerm.trim());
  }

  // Add regexp flag
  if (regexpEnabled) {
    params.set("regexp", "true");
  }

  // Add filter search term
  if (filterSearchTerm.trim()) {
    params.set("filterSearch", filterSearchTerm.trim());
  }

  // Add sticky filters
  if (stickyFilters.size > 0) {
    params.set("sticky", Array.from(stickyFilters).join(","));
  }

  const actualUrl = `${window.location.pathname}${window.location.search}`;
  const newURL = `${window.location.pathname}${params.toString() ? "?" + params.toString() : ""}`;
  if (newURL != actualUrl) {
    window.history.pushState({}, "", newURL);
  }
}

function loadStateFromURL() {
  const params = new URLSearchParams(window.location.search);

  // Load filters
  const filtersParam = params.get("filters");
  if (filtersParam) {
    try {
      const parsedFilters = JSON.parse(filtersParam);
      if (typeof parsedFilters === "object" && parsedFilters !== null) {
        filters = parsedFilters;
      }
    } catch (e) {
      console.warn("Invalid filters in URL:", e);
      filters = {};
    }
  } else {
    filters = {};
  }

  // Load search term
  const searchParam = params.get("search");
  if (searchParam) {
    searchTerm = searchParam;
    if (searchInput) {
      searchInput.value = searchTerm;
    }
  } else {
    searchTerm = "";
  }
  updateClearButton(searchTerm, searchClear);

  // Load regexp flag
  const regexpParam = params.get("regexp");
  regexpEnabled = regexpParam === "true";
  if (regexpCheckbox) {
    regexpCheckbox.checked = regexpEnabled;
  }

  // Load filter search term
  const filterSearchParam = params.get("filterSearch");
  if (filterSearchParam) {
    filterSearchTerm = filterSearchParam;
    if (filterSearchInput) {
      filterSearchInput.value = filterSearchTerm;
    }
  } else {
    filterSearchTerm = "";
  }
  updateClearButton(filterSearchTerm, filterSearchClear);

  // Load sticky filters
  const stickyParam = params.get("sticky");
  if (stickyParam) {
    stickyFilters = new Set(stickyParam.split(",").filter((s) => s.trim()));
  } else {
    stickyFilters = new Set();
  }
}

function connectWS() {
  ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onopen = () => {
    onWebsocketOpen();
    if (reconnectInterval) {
      clearInterval(reconnectInterval);
      reconnectInterval = null;
    }
  };
  ws.onclose = () => {
    renderStatus("Reconnecting...", "orange");
    if (!reconnectInterval) {
      reconnectInterval = setInterval(() => {
        renderStatus("Reconnecting...", "orange");
        connectWS();
      }, 1000);
    }
  };
  ws.onerror = () => renderStatus("Error", "red");
  ws.onmessage = (e) => handleWSMessage(JSON.parse(e.data));
}

function handleWSMessage(msg) {
  switch (msg.type) {
    case "update_index":
      Object.assign(index, msg.payload);
      renderFilters();
      break;
    case "drop_index":
      for (const k of msg.payload) delete index[k];
      renderFilters();
      break;
    case "set_logs":
      logs = msg.payload.logs;
      renderLogs();
      index = msg.payload.indexCounts;
      renderFilters();
      renderStatus();
      break;
    case "add_logs":
      logs.push(...msg.payload);
      if (logs.length > 1000) logs = logs.slice(-1000);
      renderLogs();
      renderStatus();
      break;
    case "set_status":
      serverStatus = msg.payload;
      renderStatus();
      break;
  }
}

function sendSearchRequest() {
  const payload = { filters: { ...filters } };
  if (searchTerm.trim()) {
    payload.searchTerm = searchTerm.trim();
  }
  payload.regexp = regexpEnabled;
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
  if (serverStatus && text === "Connected") {
    const memoryMB = Math.round(serverStatus.allocatedMemory / 1024 / 1024);
    statusText = `Connected | ${logs.length}/${serverStatus.logsStored} logs | ${memoryMB} MB`;
  }
  statusEl.textContent = statusText;
  statusEl.style.color = color;
}

function renderFilters() {
  filterContent.innerHTML = "";

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
  if (!filters[key]) filters[key] = [];
  const idx = filters[key].indexOf(val);
  if (idx === -1) {
    btn.classList.add("active");
    filters[key].push(val);
  } else {
    btn.classList.remove("active");
    filters[key].splice(idx, 1);
  }
  if (filters[key].length === 0) delete filters[key];
  updateURL();
  sendSearchRequest();
}

function toggleSticky(key) {
  if (stickyFilters.has(key)) {
    stickyFilters.delete(key);
  } else {
    stickyFilters.add(key);
  }
  updateURL();
  renderFilters();
}

function setupSearch() {
  let searchTimeout;
  searchInput.addEventListener("input", (e) => {
    searchTerm = e.target.value;
    updateClearButton(searchTerm, searchClear);
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      updateURL();
      sendSearchRequest();
    }, 300); // Debounce search requests
  });

  // Regexp checkbox functionality
  regexpCheckbox.addEventListener("change", (e) => {
    regexpEnabled = e.target.checked;
    updateURL();
    sendSearchRequest();
  });

  // Clear button functionality
  setupClearButton(searchInput, searchClear, () => {
    searchTerm = "";
    regexpCheckbox.checked = false;
    regexpEnabled = false;
    updateURL();
    sendSearchRequest();
  });
}

function resetAllFilters() {
  filters = {};
  // Don't reset sticky filters, only clear the active selections
  updateURL();
  sendSearchRequest();
  renderFilters();
}

function setupFilterSearch() {
  filterSearchInput.addEventListener("input", (e) => {
    filterSearchTerm = e.target.value;
    updateClearButton(filterSearchTerm, filterSearchClear);
    renderFilters();
    updateURL(true);
  });

  // Clear button functionality for filter search
  setupClearButton(filterSearchInput, filterSearchClear, () => {
    filterSearchTerm = "";
    renderFilters();
    updateURL(true);
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
      message = `<div class="message">${escapeHtmlAndHighlightSearchTerm(log.message)}</div>`;
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
          const formattedValue = formatValue(value);
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

function formatValue(value) {
  if (value === null || value === undefined) return "null";
  if (typeof value === "object") {
    return syntaxHighlightJson(value);
  }

  let stringValue = value.toString();

  return escapeHtmlAndHighlightSearchTerm(stringValue);
}

function escapeHtmlAndHighlightSearchTerm(stringValue) {
  stringValue = highlightSearchTerm(stringValue);

  // Escape HTML but preserve our highlight spans
  return stringValue
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/&lt;span class="highlight"&gt;/g, '<span class="highlight">')
    .replace(/&lt;\/span&gt;/g, "</span>");
}

function syntaxHighlightJson(json) {
  json = JSON.stringify(json, null, 4); // Pretty-print with 2-space indent

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
      return `<span class="${className}">${highlightSearchTerm(match)}</span>`;
    },
  );
}

function highlightSearchTerm(stringValue) {
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
    isResizing = true;
    e.preventDefault();
  });

  document.addEventListener("mousemove", (e) => {
    if (!isResizing) return;

    const newWidth = e.clientX - filterPanel.offsetLeft;
    const minWidth = 228;
    const maxWidth = 600;

    if (newWidth >= minWidth && newWidth <= maxWidth) {
      filterPanel.style.width = newWidth + "px";
    }
  });

  document.addEventListener("mouseup", () => {
    isResizing = false;
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
  loadStateFromURL();
  renderFilters();
  sendSearchRequest();
});

// Initialize
document.addEventListener("DOMContentLoaded", () => {
  domContentLoaded = true;
  if (wsOpen) {
    initApplication();
  }
});

function onWebsocketOpen() {
  wsOpen = true;
  if (domContentLoaded) {
    initApplication();
  }
  renderStatus();
  sendSearchRequest();
}

function initApplication() {
  loadStateFromURL();
  renderStatus();
  sendSearchRequest();
}

connectWS();
setupSearch();
setupFilterSearch();
setupResetFilters();
setupFullscreen();
setupResizer();
setupExpandJson();
updateClearButton(searchTerm, searchClear);
updateClearButton(filterSearchTerm, filterSearchClear);
