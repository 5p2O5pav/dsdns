async function loadNodes() {
  const nodes = await api('/nodes');
  const tbody = document.getElementById('nodes-table-body');
  tbody.innerHTML = nodes.map(n => `
    <tr>
      <td>${escapeHtml(n.name)}</td>
      <td>${escapeHtml(n.endpoint)}</td>
      <td><span class="status-${n.status}">${n.status}</span></td>
      <td>${n.last_heartbeat ? new Date(n.last_heartbeat).toLocaleString() : '-'}</td>
      <td>
        <button class="btn btn-secondary btn-sm edit-node" data-id="${n.id}" data-name="${escapeHtml(n.name)}" data-endpoint="${escapeHtml(n.endpoint)}">编辑</button>
        <button class="btn btn-danger btn-sm delete-node" data-id="${n.id}">删除</button>
        <button class="btn btn-secondary btn-sm sync-node" data-id="${n.id}">同步</button>
      </td>
    </tr>
  `).join('');
  document.querySelectorAll('.edit-node').forEach(btn => {
    btn.onclick = () => {
      document.getElementById('edit-node-id').value = btn.dataset.id;
      document.getElementById('edit-node-name').value = btn.dataset.name;
      document.getElementById('edit-node-endpoint').value = btn.dataset.endpoint;
      document.getElementById('node-token-display').textContent = '';
      document.getElementById('edit-node-modal').classList.add('active');
    };
  });
  document.querySelectorAll('.delete-node').forEach(btn => {
    btn.onclick = async () => {
      if (confirm('删除节点？')) {
        await api(`/nodes/${btn.dataset.id}`, { method: 'DELETE' });
        loadNodes();
      }
    };
  });
  document.querySelectorAll('.sync-node').forEach(btn => {
    btn.onclick = async () => {
      await api(`/nodes/${btn.dataset.id}/sync`, { method: 'POST' });
      alert('已触发同步');
    };
  });
}

async function addNode() {
  const name = document.getElementById('new-node-name').value.trim();
  const endpoint = document.getElementById('new-node-endpoint').value.trim();
  if (!name || !endpoint) return alert('填写完整');
  const data = await api('/nodes', { method: 'POST', body: JSON.stringify({ name, endpoint }) });
  document.getElementById('token-display').textContent = '完整 Token: ' + data.token + ' (请立即复制)';
  loadNodes();
}

async function updateNode() {
  const id = document.getElementById('edit-node-id').value;
  const name = document.getElementById('edit-node-name').value.trim();
  const endpoint = document.getElementById('edit-node-endpoint').value.trim();
  await api(`/nodes/${id}`, { method: 'PUT', body: JSON.stringify({ name, endpoint }) });
  document.getElementById('edit-node-modal').classList.remove('active');
  loadNodes();
}

async function syncAllNodes() {
  if (!confirm('确定全量同步到所有在线节点吗？')) return;
  await api('/nodes/sync-all', { method: 'POST' });
  alert('已广播同步指令');
}

function initNodes() {
  document.getElementById('module-nodes').innerHTML = `
    <div class="card">
      <div class="card-header">
        <h2>节点列表</h2>
        <button id="sync-all-nodes-btn" class="btn btn-secondary btn-sm">全量同步</button>
      </div>
      <table>
        <thead><tr><th>名称</th><th>Endpoint</th><th>状态</th><th>最后心跳</th><th>操作</th></tr></thead>
        <tbody id="nodes-table-body"></tbody>
      </table>
    </div>
    <div class="card">
      <div class="card-header"><h2>添加节点</h2></div>
      <div style="padding:1rem; display:flex; gap:1rem; align-items:flex-end;">
        <input id="new-node-name" placeholder="名称">
        <input id="new-node-endpoint" placeholder="http://10.0.0.2:9090">
        <button id="add-node-btn" class="btn btn-primary">添加</button>
      </div>
      <div id="token-display" style="padding:1rem; color:red;"></div>
    </div>
    <!-- 编辑节点模态框 -->
    <div id="edit-node-modal" class="modal-overlay">
      <div class="modal-container" style="max-width:500px;">
        <div class="modal-header"><h3>编辑节点</h3><button class="close-modal2">&times;</button></div>
        <div class="modal-body">
          <input type="hidden" id="edit-node-id">
          <label>名称</label><input id="edit-node-name" class="mb-1">
          <label>Endpoint</label><input id="edit-node-endpoint" class="mb-1">
          <div id="node-token-display"></div>
        </div>
        <div class="modal-footer">
          <button id="update-node-btn" class="btn btn-primary">保存</button>
        </div>
      </div>
    </div>
  `;
  loadNodes();
  document.getElementById('add-node-btn').onclick = addNode;
  document.getElementById('sync-all-nodes-btn').onclick = syncAllNodes;
  document.getElementById('update-node-btn').onclick = updateNode;
  document.querySelector('.close-modal2').onclick = () => document.getElementById('edit-node-modal').classList.remove('active');
}
