// tsipfs admin UI

const $ = (sel) => document.querySelector(sel);
const API = '/api/v1';
let gatewayURL = ''; // populated from status endpoint

// --- Status ---
async function refreshStatus() {
  try {
    const res = await fetch(`${API}/status`);
    const data = await res.json();
    $('#peer-id').textContent = data.peer_id || '—';
    $('#peer-count').textContent = data.peer_count;
    $('#uptime').textContent = data.uptime || '—';
    $('#pinned-size').textContent = data.pinned_size || '—';
    $('#cache-size').textContent = data.cache_size || '—';
    if (data.gateway_url) gatewayURL = data.gateway_url;
    $('#status-badge').textContent = 'online';
    $('#status-badge').className = 'badge ok';
  } catch {
    $('#status-badge').textContent = 'offline';
    $('#status-badge').className = 'badge err';
  }
}

// --- Pins ---
async function refreshPins() {
  try {
    const res = await fetch(`${API}/pins`);
    const pins = await res.json();
    const tbody = $('#pins-body');
    tbody.innerHTML = '';

    if (pins.length === 0) {
      $('#no-pins').hidden = false;
      $('#pins-table').hidden = true;
      return;
    }

    $('#no-pins').hidden = true;
    $('#pins-table').hidden = false;

    for (const pin of pins) {
      const tr = document.createElement('tr');
      const typeLabel = pin.type === 'recursive' ? 'pinned' : pin.type === 'direct' ? 'direct' : pin.type;
      const cidLink = gatewayURL
        ? `<a href="${gatewayURL}/ipfs/${pin.cid}" target="_blank" rel="noopener">${pin.cid}</a>`
        : pin.cid;
      const date = pin.pinned_at ? new Date(pin.pinned_at).toLocaleDateString() : '';
      tr.innerHTML = `
        <td>${cidLink}</td>
        <td>${pin.name || ''}</td>
        <td>${date}</td>
        <td>${typeLabel}</td>
        <td><button class="btn-unpin" data-cid="${pin.cid}">unpin</button></td>
      `;
      tbody.appendChild(tr);
    }
  } catch (err) {
    console.error('Failed to load pins:', err);
  }
}

async function unpin(cid) {
  if (!confirm(`Unpin ${cid}?`)) return;
  try {
    await fetch(`${API}/pins/${cid}`, { method: 'DELETE' });
    refreshPins();
    refreshStatus();
  } catch (err) {
    alert(`Unpin failed: ${err.message}`);
  }
}

$('#pins-body').addEventListener('click', (e) => {
  if (e.target.classList.contains('btn-unpin')) {
    unpin(e.target.dataset.cid);
  }
});

$('#refresh-pins').addEventListener('click', refreshPins);

// --- Upload ---
const dropZone = $('#drop-zone');
const fileInput = $('#file-input');

dropZone.addEventListener('click', (e) => {
  // Avoid double-triggering when clicking the <label> which already opens the input
  if (e.target.tagName === 'LABEL' || e.target === fileInput) return;
  fileInput.click();
});

dropZone.addEventListener('dragover', (e) => {
  e.preventDefault();
  dropZone.classList.add('dragover');
});

dropZone.addEventListener('dragleave', () => {
  dropZone.classList.remove('dragover');
});

dropZone.addEventListener('drop', (e) => {
  e.preventDefault();
  dropZone.classList.remove('dragover');
  if (e.dataTransfer.files.length > 0) {
    uploadFile(e.dataTransfer.files[0]);
  }
});

fileInput.addEventListener('change', () => {
  if (fileInput.files.length > 0) {
    uploadFile(fileInput.files[0]);
  }
});

function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file);

  const xhr = new XMLHttpRequest();
  const progressEl = $('#upload-progress');
  const progressBar = $('#progress-bar');
  const progressText = $('#progress-text');
  const resultEl = $('#upload-result');
  const resultCid = $('#result-cid');

  progressEl.hidden = false;
  resultEl.hidden = true;
  progressBar.style.width = '0%';
  progressText.textContent = '0%';

  xhr.upload.addEventListener('progress', (e) => {
    if (e.lengthComputable) {
      const pct = Math.round((e.loaded / e.total) * 100);
      progressBar.style.width = pct + '%';
      progressText.textContent = pct + '%';
    }
  });

  xhr.addEventListener('load', () => {
    if (xhr.status === 200) {
      const data = JSON.parse(xhr.responseText);
      progressBar.style.width = '100%';
      progressText.textContent = 'Done';
      resultEl.hidden = false;
      if (gatewayURL) {
        resultCid.innerHTML = `<a href="${gatewayURL}/ipfs/${data.cid}" target="_blank" rel="noopener">${data.cid}</a>`;
      } else {
        resultCid.textContent = data.cid;
      }
      refreshPins();
      refreshStatus();
    } else {
      progressText.textContent = 'Error: ' + xhr.responseText;
    }
    fileInput.value = '';
  });

  xhr.addEventListener('error', () => {
    progressText.textContent = 'Upload failed';
    fileInput.value = '';
  });

  xhr.open('POST', `${API}/pins`);
  xhr.send(formData);
}

// --- Bootstrap ---
let bootstrapAddrs = [];

async function refreshBootstrap() {
  try {
    const res = await fetch(`${API}/bootstrap`);
    bootstrapAddrs = await res.json();
    const list = $('#bootstrap-list');
    if (bootstrapAddrs.length === 0) {
      list.innerHTML = '<code class="mono">No known nodes yet.</code>';
      return;
    }
    list.innerHTML = bootstrapAddrs.map(a => `<code class="mono">${a}</code>`).join('');
  } catch (err) {
    console.error('Failed to load bootstrap list:', err);
  }
}

$('#copy-bootstrap').addEventListener('click', () => {
  if (bootstrapAddrs.length === 0) return;
  copyText(bootstrapAddrs.join('\n')).then(() => {
    $('#copy-bootstrap').textContent = 'Copied!';
    setTimeout(() => { $('#copy-bootstrap').textContent = 'Copy all'; }, 1500);
  });
});

function copyText(text) {
  // navigator.clipboard requires HTTPS; fall back for plain HTTP (tailnet)
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text);
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
  return Promise.resolve();
}

$('#connect-peer').addEventListener('click', async () => {
  const raw = $('#peer-multiaddr').value.trim();
  if (!raw) return;

  const addrs = raw.split('\n').map(s => s.trim()).filter(s => s && !s.startsWith('#'));
  if (addrs.length === 0) return;

  const statusEl = $('#connect-status');
  statusEl.textContent = `Adding ${addrs.length} node(s)...`;
  statusEl.className = 'connect-status';

  let added = 0;
  let errors = [];
  for (const addr of addrs) {
    try {
      const res = await fetch(`${API}/peers/connect`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ multiaddr: addr }),
      });
      if (!res.ok) {
        const text = await res.text();
        errors.push(`${addr.slice(0, 30)}...: ${text.trim()}`);
      } else {
        added++;
      }
    } catch (err) {
      errors.push(`${addr.slice(0, 30)}...: ${err.message}`);
    }
  }

  if (errors.length === 0) {
    statusEl.textContent = `Added ${added} node(s)!`;
    statusEl.className = 'connect-status ok';
    $('#peer-multiaddr').value = '';
  } else {
    statusEl.textContent = `Added ${added}, failed ${errors.length}: ${errors[0]}`;
    statusEl.className = 'connect-status err';
  }
  refreshPeers();
  refreshStatus();
  refreshBootstrap();
});

// --- Peers ---
async function refreshPeers() {
  try {
    const res = await fetch(`${API}/peers`);
    const peers = await res.json();
    const tbody = $('#peers-body');
    tbody.innerHTML = '';

    if (peers.length === 0) {
      $('#no-peers').hidden = false;
      $('#peers-table').hidden = true;
      return;
    }

    $('#no-peers').hidden = true;
    $('#peers-table').hidden = false;

    for (const peer of peers) {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${peer.id}</td>
        <td>${(peer.addrs || []).join('<br>')}</td>
      `;
      tbody.appendChild(tr);
    }
  } catch (err) {
    console.error('Failed to load peers:', err);
  }
}

$('#refresh-peers').addEventListener('click', refreshPeers);

// --- Cache ---
async function refreshCache() {
  try {
    const res = await fetch(`${API}/cache/status`);
    const data = await res.json();
    $('#cache-usage').textContent = data.size_human || '0 B';
  } catch (err) {
    console.error('Failed to load cache status:', err);
  }
}

$('#flush-cache').addEventListener('click', async () => {
  if (!confirm('Flush all cached content? Pinned content will not be affected.')) return;
  try {
    await fetch(`${API}/cache/flush`, { method: 'POST' });
    refreshCache();
    refreshStatus();
  } catch (err) {
    alert(`Flush failed: ${err.message}`);
  }
});

// --- Init ---
// Load status first (provides gatewayURL), then everything else
refreshStatus().then(() => {
  refreshPins();
  refreshPeers();
  refreshCache();
  refreshBootstrap();
});

// Auto-refresh status every 10s
setInterval(refreshStatus, 10000);
