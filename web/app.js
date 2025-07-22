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
    setStatus("Connected", "green");
    if (reconnectInterval) {
      clearInterval(reconnectInterval);
      reconnectInterval = null;
    }
  };
  ws.onclose = () => {
    setStatus("Reconnecting...", "orange");
    if (!reconnectInterval) {
      reconnectInterval = setInterval(() => {
        setStatus("Reconnecting...", "orange");
        connectWS();
      }, 1000);
    }
  };
  ws.onerror = () => setStatus("Error", "red");
  ws.onmessage = (e) => handleWSMessage(JSON.parse(e.data));
}

function setStatus(text, color) {
  let statusText = text;
  if (serverStatus && text === "Connected") {
    const memoryMB = Math.round(serverStatus.allocatedMemory / 1024 / 1024);
    statusText = `Connected | ${serverStatus.logsStored} logs | ${memoryMB} MB`;
  }
  statusEl.textContent = statusText;
  statusEl.style.color = color;
}

function handleWSMessage(msg) {
  switch (msg.type) {
    case "set_index":
      index = msg.payload;
      renderFilters();
      break;
    case "update_index":
      Object.assign(index, msg.payload);
      renderFilters();
      break;
    case "drop_index":
      for (const k of msg.payload) delete index[k];
      renderFilters();
      break;
    case "set_logs":
      logs = msg.payload;
      renderLogs();
      break;
    case "add_logs":
      logs.push(...msg.payload);
      if (logs.length > 1000) logs = logs.slice(-1000);
      renderLogs();
      break;
    case "set_status":
      serverStatus = msg.payload;
      setStatus("Connected", "green");
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
    ws.send(JSON.stringify({ type: "set_filter", payload: payload }));
  }
}

function setupSearch() {
  let searchTimeout;
  searchInput.addEventListener("input", (e) => {
    searchTerm = e.target.value;
    updateClearButton();
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      sendFilterRequest();
    }, 300); // Debounce search requests
  });

  // Clear button functionality
  searchClear.addEventListener("click", () => {
    searchInput.value = "";
    searchTerm = "";
    updateClearButton();
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

function updateClearButton() {
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
    if (log.timestamp) {
      timestamp = `<div class="timestamp">${new Date(log.timestamp).toISOString()}</div>`;
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
    const properties = Object.keys(log)
      .filter(
        (key) => !["timestamp", "level", "level_name", "message"].includes(key),
      )
      .map((key) => {
        const value = log[key];
        const valueClass = getValueClass(value);
        const formattedValue = formatValue(value);
        return `<div class="property">
          <span class="property-name">${escapeHtml(key)}:</span>
          <span class="property-value ${valueClass}">${formattedValue}</span>
        </div>`;
      })
      .join("");

    const propertiesDisplay = properties
      ? `<div class="properties">${properties}</div>`
      : "";

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
  if (value === null || value === undefined) return "null";
  return "";
}

function formatValue(value) {
  if (value === null || value === undefined) return "null";
  if (typeof value === "object") return JSON.stringify(value);
  return escapeHtml(value.toString());
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

// Initialize
connectWS();
setupSearch();
setupFilterSearch();
setupResetFilters();
setupFullscreen();
setupResizer();
updateClearButton();
updateFilterClearButton();
