const statusEl = document.getElementById('status');
const filterPanel = document.getElementById('filter-panel');
const logPanel = document.getElementById('log-panel');

let ws;
let filters = {};
let index = {};
let logs = [];

function connectWS() {
  ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onopen = () => setStatus('Connected', 'green');
  ws.onclose = () => setStatus('Disconnected', 'red');
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
      btn.onclick = () => toggleFilter(key, val);
      box.appendChild(btn);
    }
    filterPanel.appendChild(box);
  }
}

function toggleFilter(key, val) {
  if (!filters[key]) filters[key] = [];
  const idx = filters[key].indexOf(val);
  if (idx === -1) filters[key].push(val);
  else filters[key].splice(idx, 1);
  if (filters[key].length === 0) delete filters[key];
  ws.send(JSON.stringify({ type: 'set_filter', payload: filters }));
}

function renderLogs() {
  logPanel.innerHTML = '';
  for (const log of logs) {
    const pre = document.createElement('pre');
    pre.className = 'log-entry';
    pre.innerHTML = syntaxHighlight(log);
    logPanel.appendChild(pre);
  }
  logPanel.scrollTop = logPanel.scrollHeight;
}

function syntaxHighlight(json) {
  if (typeof json != 'string') json = JSON.stringify(json, null, 2);
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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
    return `<span class="${cls}">${match}</span>`;
  });
}

document.addEventListener('DOMContentLoaded', connectWS); 