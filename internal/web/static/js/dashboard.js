let dashboardInterval = null;

async function loadDashboardStats() {
    const stats = await api('/dashboard/stats');
    document.getElementById('stat-domains').textContent = stats.total_domains;
    document.getElementById('stat-records').textContent = stats.total_records;
    // 只有管理员才显示节点统计卡片
    if (isAdmin()) {
        document.getElementById('stat-online').textContent = stats.online_nodes;
        document.getElementById('stat-offline').textContent = stats.offline_nodes;
        document.getElementById('stat-notify').textContent = stats.today_notifications;
    } else {
        // 隐藏节点卡片
        const nodeCards = document.querySelectorAll('.node-stat-card');
        nodeCards.forEach(card => card.style.display = 'none');
        document.getElementById('stat-notify').textContent = stats.today_notifications;
    }
}

async function loadNotifications() {
    const notifs = await api('/dashboard/recent-notifications?limit=20');
    const tbody = document.getElementById('notifications-tbody');
    tbody.innerHTML = notifs.map(n => `
        <tr>
            <td>${n.type === 'record_invalid' ? '记录告警' : (n.type === 'node_offline' ? '节点离线' : '节点恢复')}</td>
            <td>${escapeHtml(n.content)}</td>
            <td>${new Date(n.created_at).toLocaleString()}</td>
        </tr>
    `).join('');
}

async function loadHealthTree() {
    const data = await api('/dashboard/health-tree');
    const container = document.getElementById('health-tree-container');
    container.innerHTML = ''; // 清空

    if (isAdmin()) {
        // 管理员数据格式：[{user_id, username, domains: [{domain_id, domain_name, records:[]}]}]
        for (const user of data) {
            const userDiv = document.createElement('div');
            userDiv.className = 'health-user';
            const userHeader = document.createElement('div');
            userHeader.className = 'health-user-header';
            userHeader.textContent = `👤 ${escapeHtml(user.username)} (${user.domains.length} 个域名)`;
            userHeader.style.cursor = 'pointer';
            userHeader.style.fontWeight = 'bold';
            userHeader.style.padding = '0.5rem';
            userHeader.style.background = '#f1f5f9';
            userHeader.style.marginTop = '0.5rem';
            userHeader.style.borderRadius = '0.5rem';
            const userBody = document.createElement('div');
            userBody.className = 'health-user-body';
            userBody.style.display = 'none';
            userBody.style.marginLeft = '1rem';

            for (const domain of user.domains) {
                const domainDiv = document.createElement('div');
                const domainHeader = document.createElement('div');
                domainHeader.textContent = `🌐 ${escapeHtml(domain.domain_name)} (${domain.records.length} 条记录)`;
                domainHeader.style.cursor = 'pointer';
                domainHeader.style.padding = '0.3rem';
                domainHeader.style.background = '#e2e8f0';
                domainHeader.style.margin = '0.2rem 0';
                domainHeader.style.borderRadius = '0.3rem';
                const domainBody = document.createElement('div');
                domainBody.style.display = 'none';
                domainBody.style.marginLeft = '1rem';

                if (domain.records.length === 0) {
                    domainBody.textContent = '暂无记录';
                } else {
                    const table = document.createElement('table');
                    table.style.width = '100%';
                    table.style.borderCollapse = 'collapse';
                    table.innerHTML = `<thead><tr><th>类型</th><th>值</th><th>成功率</th><th>最后检查</th></tr></thead><tbody></tbody>`;
                    const tbody = table.querySelector('tbody');
                    for (const rec of domain.records) {
                        const rate = (rec.success_rate * 100).toFixed(1);
                        const rateCls = rec.success_rate < 0.8 ? 'style="color:red;"' : '';
                        const row = tbody.insertRow();
                        row.insertCell(0).textContent = rec.type;
                        row.insertCell(1).textContent = rec.value;
                        row.insertCell(2).innerHTML = `<span ${rateCls}>${rate}%</span>`;
                        row.insertCell(3).textContent = rec.last_check ? new Date(rec.last_check).toLocaleString() : '-';
                    }
                    domainBody.appendChild(table);
                }

                domainHeader.onclick = () => {
                    const isVisible = domainBody.style.display !== 'none';
                    domainBody.style.display = isVisible ? 'none' : 'block';
                };
                domainDiv.appendChild(domainHeader);
                domainDiv.appendChild(domainBody);
                userBody.appendChild(domainDiv);
            }

            userHeader.onclick = () => {
                const isVisible = userBody.style.display !== 'none';
                userBody.style.display = isVisible ? 'none' : 'block';
            };
            userDiv.appendChild(userHeader);
            userDiv.appendChild(userBody);
            container.appendChild(userDiv);
        }
    } else {
        // 普通用户数据格式：[{domain_id, domain_name, records:[]}]
        for (const domain of data) {
            const domainDiv = document.createElement('div');
            const domainHeader = document.createElement('div');
            domainHeader.textContent = `🌐 ${escapeHtml(domain.domain_name)} (${domain.records.length} 条记录)`;
            domainHeader.style.cursor = 'pointer';
            domainHeader.style.padding = '0.5rem';
            domainHeader.style.background = '#e2e8f0';
            domainHeader.style.margin = '0.5rem 0';
            domainHeader.style.borderRadius = '0.5rem';
            domainHeader.style.fontWeight = 'bold';
            const domainBody = document.createElement('div');
            domainBody.style.display = 'none';
            domainBody.style.marginLeft = '1rem';

            if (domain.records.length === 0) {
                domainBody.textContent = '暂无记录';
            } else {
                const table = document.createElement('table');
                table.style.width = '100%';
                table.style.borderCollapse = 'collapse';
                table.innerHTML = `<thead><tr><th>类型</th><th>值</th><th>成功率</th><th>最后检查</th></tr></thead><tbody></tbody>`;
                const tbody = table.querySelector('tbody');
                for (const rec of domain.records) {
                    const rate = (rec.success_rate * 100).toFixed(1);
                    const rateCls = rec.success_rate < 0.8 ? 'style="color:red;"' : '';
                    const row = tbody.insertRow();
                    row.insertCell(0).textContent = rec.type;
                    row.insertCell(1).textContent = rec.value;
                    row.insertCell(2).innerHTML = `<span ${rateCls}>${rate}%</span>`;
                    row.insertCell(3).textContent = rec.last_check ? new Date(rec.last_check).toLocaleString() : '-';
                }
                domainBody.appendChild(table);
            }

            domainHeader.onclick = () => {
                const isVisible = domainBody.style.display !== 'none';
                domainBody.style.display = isVisible ? 'none' : 'block';
            };
            domainDiv.appendChild(domainHeader);
            domainDiv.appendChild(domainBody);
            container.appendChild(domainDiv);
        }
    }
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
    await loadDashboardStats();
    await loadNotifications();
    await loadHealthTree();
}

function initDashboard() {
    const statsCardsHtml = isAdmin() ? `
        <div class="stat-card"><div class="stat-number" id="stat-domains">0</div><div class="stat-label">域名数</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-records">0</div><div class="stat-label">记录数</div></div>
        <div class="stat-card node-stat-card"><div class="stat-number status-online" id="stat-online">0</div><div class="stat-label">在线节点</div></div>
        <div class="stat-card node-stat-card"><div class="stat-number status-offline" id="stat-offline">0</div><div class="stat-label">离线节点</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-notify">0</div><div class="stat-label">今日告警</div></div>
    ` : `
        <div class="stat-card"><div class="stat-number" id="stat-domains">0</div><div class="stat-label">域名数</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-records">0</div><div class="stat-label">记录数</div></div>
        <div class="stat-card"><div class="stat-number" id="stat-notify">0</div><div class="stat-label">今日告警</div></div>
    `;

    document.getElementById('module-dashboard').innerHTML = `
        <div class="card">
            <div class="card-header"><h2>总览</h2></div>
            <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(150px,1fr)); gap:1rem; padding:1.5rem;">
                ${statsCardsHtml}
            </div>
        </div>
        <div class="card">
            <div class="card-header"><h2>最近告警</h2></div>
            <table><thead><tr><th>类型</th><th>内容</th><th>时间</th></tr></thead><tbody id="notifications-tbody"></tbody></table>
        </div>
        <div class="card">
            <div class="card-header"><h2>记录健康度</h2></div>
            <div id="health-tree-container" style="padding:1rem;"></div>
        </div>
    `;
    // 添加统计卡片样式（确保存在）
    if (!document.getElementById('stat-card-style')) {
        const style = document.createElement('style');
        style.id = 'stat-card-style';
        style.textContent = `.stat-card { background: #f8fafc; border-radius:0.75rem; padding:1rem; text-align:center; } .stat-number { font-size:2rem; font-weight:700; } .stat-label { color:#64748b; } .health-user-header, .health-domain-header { user-select: none; }`;
        document.head.appendChild(style);
    }
    startDashboardAutoRefresh();
    window.stopDashboard = stopDashboardAutoRefresh;
}
