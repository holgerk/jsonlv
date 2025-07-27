const statusEl = document.getElementById("status");
const filterPanel = document.getElementById("filter-panel");
const filterContent = document.getElementById("filter-content");
const resizer = document.getElementById("resizer");
const logPanel = document.getElementById("log-panel");
const searchInput = document.getElementById("search-input");
const searchClear = document.getElementById("search-clear");
const filterSearchInput = document.getElementById("filter-search-input");
const filterSearchClear = document.getElementById("filter-search-clear");
const resetFiltersBtn = document.getElementById("reset-filters");

let ws;
let reconnectInterval = null;
let filters = {};
let searchTerm = "";
let filterSearchTerm = "";
let index = {};
let logs = [];
let serverStatus = null;

let isResizing = false;

function connectWS() {
  ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onopen = () => {
    renderStatus();
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

function renderFilters() {
  filterContent.innerHTML = "";

  // Filter the index keys based on filterSearchTerm
  const filteredKeys = Object.keys(index).filter((key) => {
    if (!filterSearchTerm.trim()) return true;
    return key.toLowerCase().includes(filterSearchTerm.toLowerCase());
  });

  for (const key of filteredKeys) {
    const box = document.createElement("div");
    box.className = "filter-box";
    const title = document.createElement("div");
    title.className = "filter-title";
    title.textContent = key;
    box.appendChild(title);
    const values = index[key];
    for (const val in values) {
      const btn = document.createElement("button");
      btn.textContent = `${val} (${values[val]})`;
      btn.className = "filter-btn";
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
  sendFilterRequest();
}

function sendFilterRequest() {
  const payload = { filters: { ...filters } };
  if (searchTerm.trim()) {
    payload.searchTerm = searchTerm.trim();
  }
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: "set_search", payload: payload }));
  }
}

function setupSearch() {
  let searchTimeout;
  searchInput.addEventListener("input", (e) => {
    searchTerm = e.target.value;
    updateSearchClearButton();
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      sendFilterRequest();
    }, 300); // Debounce search requests
  });

  // Clear button functionality
  searchClear.addEventListener("click", () => {
    searchInput.value = "";
    searchTerm = "";
    updateSearchClearButton();
    sendFilterRequest();
  });
}

function resetAllFilters() {
  filters = {};
  sendFilterRequest();
  renderFilters();
}

function setupFilterSearch() {
  filterSearchInput.addEventListener("input", (e) => {
    filterSearchTerm = e.target.value;
    updateFilterClearButton();
    renderFilters();
  });

  // Clear button functionality for filter search
  filterSearchClear.addEventListener("click", () => {
    filterSearchInput.value = "";
    filterSearchTerm = "";
    updateFilterClearButton();
    renderFilters();
  });
}

function updateSearchClearButton() {
  if (searchTerm.trim()) {
    searchClear.classList.add("visible");
  } else {
    searchClear.classList.remove("visible");
  }
}

function updateFilterClearButton() {
  if (filterSearchTerm.trim()) {
    filterSearchClear.classList.add("visible");
  } else {
    filterSearchClear.classList.remove("visible");
  }
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
      message = `<div class="message">${escapeHtml(log.message)}</div>`;
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

  stringValue = highlightSearchTerm(stringValue);

  // Escape HTML but preserve our highlight spans
  stringValue = stringValue
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/&lt;span class="highlight"&gt;/g, '<span class="highlight">')
    .replace(/&lt;\/span&gt;/g, "</span>");

  return stringValue;
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

// Initialize
connectWS();
setupSearch();
setupFilterSearch();
setupResetFilters();
setupFullscreen();
setupResizer();
setupExpandJson();
updateSearchClearButton();
updateFilterClearButton();
