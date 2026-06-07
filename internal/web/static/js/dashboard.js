let dashboardInterval = null;

async function loadDashboardStats() {
  const stats = await api('/dashboard/stats');
  document.getElementById('stat-domains').textContent = stats.total_domains;
  document.getElementById('stat-records').textContent = stats.total_records;
  document.getElementById('stat-online').textContent = stats.online_nodes;
  document.getElementById('stat-offline').textContent = stats.offline_nodes;
  document.getElementById('stat-notify').textContent = stats.today_notifications;
}

async function loadNotifications() {
  const notifs = await api('/dashboard/recent-notifications?limit=20');
  const tbody = document.getElementById('notifications-tbody');
  tbody.innerHTML = notifs.map(n => `
    <tr>
      <td>${n.type}</td>
      <td>${escapeHtml(n.content)}</td>
      <td>${n.is_resolved ? '已解决' : '未解决'}</td>
      <td>${new Date(n.created_at).toLocaleString()}</td>
    </tr>
  `).join('');
}

async function loadHealth() {
  const health = await api('/dashboard/record-health');
  const tbody = document.getElementById('health-tbody');
  tbody.innerHTML = health.map(h => {
    const rate = (h.success_rate * 100).toFixed(1);
    const cls = h.success_rate < 0.8 ? 'style="color:red;"' : '';
    return `<tr>
      <td>${escapeHtml(h.domain.replace(/\.$/, ''))}</td>
      <td>${h.type}</td>
      <td>${h.value}</td>
      <td ${cls}>${rate}%</td>
      <td>${h.last_check ? new Date(h.last_check).toLocaleString() : '-'}</td>
    </tr>`;
  }).join('');
}

function startDashboardAutoRefresh() {
  stopDashboardAutoRefresh();
  loadDashboardData();
  dashboardInterval = setInterval(loadDashboardData, 10000);
}

function stopDashboardAutoRefresh() {
  if (dashboardInterval) {
    clearInterval(dashboardInterval);
    dashboardInterval = null;
  }
}

async function loadDashboardData() {
  await Promise.all([loadDashboardStats(), loadNotifications(), loadHealth()]);
}

function initDashboard() {
  document.getElementById('module-dashboard').innerHTML = `
    <div class="card">
      <div class="card-header"><h2>总览</h2></div>
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(150px,1fr)); gap:1rem; padding:1.5rem;">
        <div class="stat-card"><div class="stat-number" id="stat-domains">0</div><div class="stat-label">域名数</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-records">0</div><div class="stat-label">记录数</div></div>
        <div class="stat-card"><div class="stat-number status-online" id="stat-online">0</div><div class="stat-label">在线节点</div></div>
        <div class="stat-card"><div class="stat-number status-offline" id="stat-offline">0</div><div class="stat-label">离线节点</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-notify">0</div><div class="stat-label">今日告警</div></div>
      </div>
    </div>
    <div class="card">
      <div class="card-header"><h2>最近告警</h2></div>
      <table><thead><tr><th>类型</th><th>内容</th><th>状态</th><th>时间</th></tr></thead><tbody id="notifications-tbody"></tbody></table>
    </div>
    <div class="card">
      <div class="card-header"><h2>记录健康度</h2></div>
      <table><thead><tr><th>域名</th><th>类型</th><th>值</th><th>成功率</th><th>最后检查</th></tr></thead><tbody id="health-tbody"></tbody></table>
    </div>
  `;
  // 添加统计卡片样式
  if (!document.getElementById('stat-card-style')) {
    const style = document.createElement('style');
    style.id = 'stat-card-style';
    style.textContent = `.stat-card { background: #f8fafc; border-radius:0.75rem; padding:1rem; text-align:center; } .stat-number { font-size:2rem; font-weight:700; } .stat-label { color:#64748b; }`;
    document.head.appendChild(style);
  }
  startDashboardAutoRefresh();
  // 当模块隐藏时停止（由 switchModule 控制）
  window.stopDashboard = stopDashboardAutoRefresh;
}
