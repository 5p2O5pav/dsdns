async function loadUsers() {
  const users = await api('/users');
  const tbody = document.getElementById('users-tbody');
  tbody.innerHTML = users.map(u => `
    <tr>
      <td>${u.id}</td>
      <td>${escapeHtml(u.username)}</td>
      <td>${u.is_admin ? '管理员' : '普通用户'}</td>
      <td>${new Date(u.created_at).toLocaleString()}</td>
      <td>
        ${!u.is_admin ? `<button class="btn btn-danger btn-sm delete-user" data-id="${u.id}">删除</button>` : ''}
      </td>
    </tr>
  `).join('');
  document.querySelectorAll('.delete-user').forEach(btn => {
    btn.onclick = async () => {
      if (confirm('删除用户？')) {
        await api(`/users/${btn.dataset.id}`, { method: 'DELETE' });
        loadUsers();
      }
    };
  });
}

async function addUser() {
  const username = document.getElementById('new-username').value.trim();
  const password = document.getElementById('new-password').value;
  if (!username || !password) return alert('请填写完整');
  await api('/users', { method: 'POST', body: JSON.stringify({ username, password }) });
  document.getElementById('new-username').value = '';
  document.getElementById('new-password').value = '';
  loadUsers();
}

function initUsers() {
  document.getElementById('module-users').innerHTML = `
    <div class="card">
      <div class="card-header"><h2>用户列表</h2></div>
      <table><thead><tr><th>ID</th><th>用户名</th><th>角色</th><th>创建时间</th><th>操作</th></tr></thead><tbody id="users-tbody"></tbody></table>
    </div>
    <div class="card">
      <div class="card-header"><h2>添加用户</h2></div>
      <div style="padding:1rem; display:flex; gap:1rem;">
        <input id="new-username" placeholder="用户名">
        <input type="password" id="new-password" placeholder="密码">
        <button id="add-user-btn" class="btn btn-primary">添加</button>
      </div>
    </div>
  `;
  loadUsers();
  document.getElementById('add-user-btn').onclick = addUser;
}
