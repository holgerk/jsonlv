const statusEl = document.getElementById('status');
const filterPanel = document.getElementById('filter-panel');
const resizer = document.getElementById('resizer');
const logPanel = document.getElementById('log-panel');
const searchInput = document.getElementById('search-input');

let ws;
let reconnectInterval = null;
let filters = {};
let searchTerm = '';
let index = {};
let logs = [];

let isResizing = false;

function connectWS() {
  ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onopen = () => {
    setStatus('Connected', 'green');
    if (reconnectInterval) {
      clearInterval(reconnectInterval);
      reconnectInterval = null;
    }
  };
  ws.onclose = () => {
    setStatus('Reconnecting...', 'orange');
    if (!reconnectInterval) {
      reconnectInterval = setInterval(() => {
        setStatus('Reconnecting...', 'orange');
        connectWS();
      }, 1000);
    }
  };
  ws.onerror = () => setStatus('Error', 'red');
  ws.onmessage = (e) => handleWSMessage(JSON.parse(e.data));
}

function setStatus(text, color) {
  statusEl.textContent = text;
  statusEl.style.color = color;
}

function handleWSMessage(msg) {
  switch (msg.type) {
    case 'set_index':
      index = msg.payload;
      renderFilters();
      break;
    case 'update_index':
      Object.assign(index, msg.payload);
      renderFilters();
      break;
    case 'drop_index':
      for (const k of msg.payload) delete index[k];
      renderFilters();
      break;
    case 'set_logs':
      logs = msg.payload;
      renderLogs();
      break;
    case 'add_logs':
      logs.push(...msg.payload);
      if (logs.length > 1000) logs = logs.slice(-1000);
      renderLogs();
      break;
  }
}

function renderFilters() {
  filterPanel.innerHTML = '';
  for (const key in index) {
    const box = document.createElement('div');
    box.className = 'filter-box';
    const title = document.createElement('div');
    title.className = 'filter-title';
    title.textContent = key;
    box.appendChild(title);
    const values = index[key];
    for (const val in values) {
      const btn = document.createElement('button');
      btn.textContent = `${val} (${values[val]})`;
      btn.className = 'filter-btn';
      if (filters[key] && filters[key].includes(val)) btn.classList.add('active');
      btn.onclick = function () { toggleFilter(this, key, val); };
      box.appendChild(btn);
    }
    filterPanel.appendChild(box);
  }
}

function toggleFilter(btn, key, val) {
  if (!filters[key]) filters[key] = [];
  const idx = filters[key].indexOf(val);
  if (idx === -1) {
    btn.classList.add('active');
    filters[key].push(val);
  } else {
    btn.classList.remove('active');
    filters[key].splice(idx, 1);
  }
  if (filters[key].length === 0) delete filters[key];
  sendFilterRequest();
}

function sendFilterRequest() {
  const payload = { ...filters };
  if (searchTerm.trim()) {
    payload.searchTerm = searchTerm.trim();
  }
  ws.send(JSON.stringify({ type: 'set_filter', payload: payload }));
}

function setupSearch() {
  let searchTimeout;
  searchInput.addEventListener('input', (e) => {
    searchTerm = e.target.value;
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      sendFilterRequest();
    }, 300); // Debounce search requests
  });
}

function renderLogs() {
  // Check if user is at the bottom before rendering
  const isAtBottom = logPanel.scrollTop + logPanel.clientHeight >= logPanel.scrollHeight - 5;
  logPanel.innerHTML = '';
  for (const log of logs) {
    const pre = document.createElement('pre');
    pre.className = 'log-entry';
    pre.innerHTML = syntaxHighlight(log);
    logPanel.appendChild(pre);
  }
  // Only auto-scroll if user was at the bottom
  if (isAtBottom) {
    logPanel.scrollTop = logPanel.scrollHeight;
  }
}

function syntaxHighlight(json) {
  if (typeof json != 'string') json = JSON.stringify(json, null, 2);
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  
  // First, highlight log levels with colors and bold (case-insensitive)
  const errorLevels = ['error', 'critical'];
  const warnLevels = ['warn', 'warning'];
  const infoLevels = ['info', 'debug', 'trace'];
  
  // Then apply standard syntax highlighting
  return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|\d+)/g, function (match) {
    let cls = 'number';
    if (/^"/.test(match)) {
      if (/:$/.test(match)) cls = 'key';
      else cls = 'string';
    } else if (/true|false/.test(match)) {
      cls = 'boolean';
    } else if (/null/.test(match)) {
      cls = 'null';
    }
    if (cls === 'string') {
        for (const level of errorLevels) {
            if (`"${level.toUpperCase()}"` === match || `"${level}"` === match) {
                cls += ' log-level-error';
                match = `<strong>${match}</strong>`;
            }
        }
        for (const level of warnLevels) {
            if (`"${level.toUpperCase()}"` === match || `"${level}"` === match) {
                cls += ' log-level-warn';
                match = `<strong>${match}</strong>`;
            }
        }
        for (const level of infoLevels) {
            if (`"${level.toUpperCase()}"` === match || `"${level}"` === match) {
                cls += ' log-level-info';
                match = `<strong>${match}</strong>`;
            }
        }
    }
    else if (cls === 'key' && match === '"message":') {
        match = `<strong>${match}</strong>`;
    }
    return `<span class="${cls}">${match}</span>`;
  });
}

document.addEventListener('DOMContentLoaded', connectWS);

resizer.addEventListener('mousedown', function(e) {
  isResizing = true;
  document.body.style.cursor = 'ew-resize';
});

document.addEventListener('mousemove', function(e) {
  if (!isResizing) return;
  const minWidth = 120;
  const maxWidth = 600;
  const mainRect = document.getElementById('main').getBoundingClientRect();
  let newWidth = e.clientX - mainRect.left;
  if (newWidth < minWidth) newWidth = minWidth;
  if (newWidth > maxWidth) newWidth = maxWidth;
  filterPanel.style.width = newWidth + 'px';
});

document.addEventListener('mouseup', function(e) {
  if (isResizing) {
    isResizing = false;
    document.body.style.cursor = '';
  }
});

function setupFullscreen() {
  const fullscreenBtn = document.getElementById('fullscreen-toggle');
  fullscreenBtn.addEventListener('click', toggleFullscreen);
  
  // Update button text based on current state
  document.addEventListener('fullscreenchange', updateFullscreenButton);
  document.addEventListener('webkitfullscreenchange', updateFullscreenButton);
  document.addEventListener('mozfullscreenchange', updateFullscreenButton);
  document.addEventListener('MSFullscreenChange', updateFullscreenButton);
}

function toggleFullscreen() {
  if (!document.fullscreenElement && 
      !document.webkitFullscreenElement && 
      !document.mozFullScreenElement && 
      !document.msFullscreenElement) {
    // Enter fullscreen
    if (document.documentElement.requestFullscreen) {
      document.documentElement.requestFullscreen();
    } else if (document.documentElement.webkitRequestFullscreen) {
      document.documentElement.webkitRequestFullscreen();
    } else if (document.documentElement.mozRequestFullScreen) {
      document.documentElement.mozRequestFullScreen();
    } else if (document.documentElement.msRequestFullscreen) {
      document.documentElement.msRequestFullscreen();
    }
  } else {
    // Exit fullscreen
    if (document.exitFullscreen) {
      document.exitFullscreen();
    } else if (document.webkitExitFullscreen) {
      document.webkitExitFullscreen();
    } else if (document.mozCancelFullScreen) {
      document.mozCancelFullScreen();
    } else if (document.msExitFullscreen) {
      document.msExitFullscreen();
    }
  }
}

function updateFullscreenButton() {
  const fullscreenBtn = document.getElementById('fullscreen-toggle');
  const isFullscreen = !!(document.fullscreenElement || 
                          document.webkitFullscreenElement || 
                          document.mozFullScreenElement || 
                          document.msFullscreenElement);
  
  fullscreenBtn.textContent = isFullscreen ? '⛶' : '⛶';
  fullscreenBtn.title = isFullscreen ? 'Exit Fullscreen' : 'Enter Fullscreen';
}

// Initialize fullscreen functionality
setupFullscreen();
setupSearch(); 