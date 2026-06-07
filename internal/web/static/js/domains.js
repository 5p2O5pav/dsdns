let currentDomain = null;
let rules = [];

const CONTINENTS = {
  'asia': '亚洲', 'europe': '欧洲', 'africa': '非洲',
  'north_america': '北美', 'south_america': '南美', 'oceania': '大洋洲'
};
const ISPS = {
  'mobile': '移动', 'unicom': '联通', 'telecom': '电信',
  'broadcast': '广电', 'education': '教育网', 'other': '其他'
};
const TYPES = ['A', 'AAAA', 'CNAME'];
const TTLS = [60, 300, 600, 1200, 3600];

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/[&<>]/g, m => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[m] || m));
}

function renderRules() {
  const container = document.getElementById('rules-container');
  if (!rules.length) {
    container.innerHTML = '<div class="empty-state">暂无规则</div>';
    return;
  }
  let html = '';
  rules.forEach((rule, idx) => {
    html += `<div class="rule-row" data-idx="${idx}">`;
    html += `<div class="rule-field"><select class="rule-type" data-idx="${idx}">
      <option value="default" ${rule.rule_type==='default'?'selected':''}>默认</option>
      <option value="china_other" ${rule.rule_type==='china_other'?'selected':''}>中国其他</option>
      <option value="continent" ${rule.rule_type==='continent'?'selected':''}>大洲</option>
      <option value="china" ${rule.rule_type==='china'?'selected':''}>中国大陆</option>
    </select></div>`;
    if (rule.rule_type === 'continent') {
      html += `<div class="rule-field"><select class="continent" data-idx="${idx}">${
        Object.entries(CONTINENTS).map(([k,v])=>`<option value="${k}" ${rule.continent===k?'selected':''}>${v}</option>`).join('')
      }</select></div>`;
    }
    if (rule.rule_type === 'china') {
      html += `<div class="rule-field"><select class="china-isp" data-idx="${idx}">
        <option value="">任意运营商</option>${
        Object.entries(ISPS).map(([k,v])=>`<option value="${k}" ${rule.isp===k?'selected':''}>${v}</option>`).join('')
      }</select></div>`;
      html += `<div class="rule-field"><input class="china-province" data-idx="${idx}" placeholder="省份" value="${escapeHtml(rule.province)}"></div>`;
    }
    html += `<div class="rule-field"><select class="record-type" data-idx="${idx}">${
      TYPES.map(t=>`<option value="${t}" ${rule.type===t?'selected':''}>${t}</option>`).join('')
    }</select></div>`;
    html += `<div class="rule-field"><input class="record-value" data-idx="${idx}" value="${escapeHtml(rule.value)}" placeholder="记录值"></div>`;
    html += `<div class="rule-field"><select class="record-ttl" data-idx="${idx}">${
      TTLS.map(t=>`<option value="${t}" ${rule.ttl===t?'selected':''}>${t}秒</option>`).join('')
    }</select></div>`;
    html += `<button class="delete-rule" data-idx="${idx}">删除</button>`;
    html += `</div>`;
  });
  container.innerHTML = html;
}

function syncRulesFromDOM() {
  const rows = document.querySelectorAll('#rules-container .rule-row');
  const newRules = [];
  rows.forEach(row => {
    const idx = parseInt(row.dataset.idx);
    const ruleType = row.querySelector('.rule-type')?.value || 'default';
    let continent = '', isp = '', province = '';
    if (ruleType === 'continent') continent = row.querySelector('.continent')?.value || '';
    else if (ruleType === 'china') {
      isp = row.querySelector('.china-isp')?.value || '';
      province = row.querySelector('.china-province')?.value || '';
    }
    const type = row.querySelector('.record-type')?.value || 'A';
    const value = row.querySelector('.record-value')?.value || '';
    const ttl = parseInt(row.querySelector('.record-ttl')?.value) || 600;
    newRules.push({ rule_type: ruleType, continent, isp, province, type, value, ttl });
  });
  rules = newRules;
}

function validateRules() {
  if (!rules.some(r => r.rule_type === 'default')) {
    alert('必须保留一条默认规则');
    return false;
  }
  for (let i=0;i<rules.length;i++) {
    const r = rules[i];
    if (!r.type || !r.value || !r.ttl) {
      alert(`第${i+1}条规则不完整`);
      return false;
    }
    if (r.rule_type === 'continent' && !r.continent) {
      alert('大洲规则必须选择大洲');
      return false;
    }
  }
  return true;
}

async function saveRecords() {
  syncRulesFromDOM();
  if (!validateRules()) return;
  await api(`/domains/${currentDomain.id}`, { method: 'PUT', body: JSON.stringify(rules) });
  alert('保存成功');
  closeEditor();
  loadDomains();
}

async function renameDomain() {
  const newDomain = document.getElementById('domain-name-input').value.trim();
  if (!newDomain || newDomain === currentDomain.domain) return;
  await api(`/domains/${currentDomain.id}`, { method: 'PATCH', body: JSON.stringify({ domain: newDomain }) });
  currentDomain.domain = newDomain;
  alert('域名已修改');
  loadDomains();
}

async function openEditor(domainId, domainName) {
  try {
    currentDomain = { id: domainId, domain: domainName };
    document.getElementById('domain-name-input').value = domainName;
    const records = await api(`/domains/${domainId}`);
    rules = records.map(r => ({
      rule_type: r.rule_type,
      continent: r.continent || '',
      isp: r.isp || '',
      province: r.province || '',
      type: r.type,
      value: r.value,
      ttl: r.ttl || 600
    }));
    renderRules();
    document.getElementById('editor-modal').classList.add('active');
  } catch (err) {
    console.error('打开编辑器失败:', err);
    alert('加载域名记录失败: ' + (err.message || '未知错误'));
  }
}

function closeEditor() {
  document.getElementById('editor-modal').classList.remove('active');
  currentDomain = null;
  rules = [];
}

function addNewRule() {
  rules.push({ rule_type: 'default', continent: '', isp: '', province: '', type: 'A', value: '', ttl: 600 });
  renderRules();
}

async function loadDomains() {
  const domains = await api('/domains');
  const container = document.getElementById('domain-list');
  if (!domains.length) {
    container.innerHTML = '<div class="empty-state">暂无域名</div>';
    return;
  }
  container.innerHTML = domains.map(d => `
    <div class="domain-item flex-between" style="padding:1rem 1.5rem; border-bottom:1px solid #f0f2f5;">
      <span class="domain-name">${escapeHtml(d.domain.replace(/\.$/, ''))}</span>
      <div class="action-group">
        <button class="btn btn-secondary btn-sm edit-domain" data-id="${d.id}" data-domain="${escapeHtml(d.domain.replace(/\.$/, ''))}">编辑记录</button>
        <button class="btn btn-danger btn-sm delete-domain" data-id="${d.id}">删除</button>
      </div>
    </div>
  `).join('');
  document.querySelectorAll('.edit-domain').forEach(btn => {
    btn.onclick = () => openEditor(parseInt(btn.dataset.id), btn.dataset.domain);
  });
  document.querySelectorAll('.delete-domain').forEach(btn => {
    btn.onclick = async () => {
      if (confirm('确认删除？')) {
        await api(`/domains/${btn.dataset.id}`, { method: 'DELETE' });
        loadDomains();
      }
    };
  });
}

async function addDomain() {
  let domain = prompt('输入域名（如 example.com）');
  if (!domain) return;
  await api('/domains', { method: 'POST', body: JSON.stringify({ domain }) });
  loadDomains();
}

async function syncAllNodes() {
  if (!confirm('确定全量同步到所有在线节点吗？')) return;
  await api('/nodes/sync-all', { method: 'POST' });
  alert('已广播同步指令');
}

function initDomains() {
  const html = `
    <div class="card">
      <div class="card-header">
        <h2>域名列表</h2>
        <div style="display:flex; gap:0.5rem;">
          ${isAdmin() ? '<button id="sync-all-domains-btn" class="btn btn-secondary btn-sm">全量同步</button>' : ''}
          <button id="add-domain-btn" class="btn btn-primary">添加域名</button>
        </div>
      </div>
      <div id="domain-list">加载中...</div>
    </div>

    <div id="editor-modal" class="modal-overlay">
      <div class="modal-container">
        <div class="modal-header">
          <h3>编辑 DNS 记录</h3>
          <button class="close-modal">&times;</button>
        </div>
        <div class="modal-body">
          <div class="mb-1">
            <label>域名</label>
            <input id="domain-name-input" style="width:200px;">
            <button id="rename-domain-btn" class="btn btn-secondary btn-sm">修改</button>
          </div>
          <hr>
          <div id="rules-container"></div>
          <button id="add-rule-btn" class="btn btn-secondary mt-1">+ 添加规则</button>
        </div>
        <div class="modal-footer">
          <button id="cancel-editor" class="btn btn-secondary">取消</button>
          <button id="save-editor" class="btn btn-primary">保存</button>
        </div>
      </div>
    </div>
  `;
  document.getElementById('module-domains').innerHTML = html;
  loadDomains();

  document.getElementById('add-domain-btn').onclick = addDomain;
  document.getElementById('rename-domain-btn').onclick = renameDomain;
  document.getElementById('save-editor').onclick = saveRecords;
  document.getElementById('cancel-editor').onclick = closeEditor;
  document.querySelector('.close-modal').onclick = closeEditor;
  document.getElementById('add-rule-btn').onclick = addNewRule;
  document.getElementById('editor-modal').onclick = (e) => { if (e.target.id==='editor-modal') closeEditor(); };

  if (isAdmin()) {
    document.getElementById('sync-all-domains-btn').onclick = syncAllNodes;
  }

  // 规则编辑器事件委托
  document.getElementById('rules-container').addEventListener('change', (e) => {
    const row = e.target.closest('.rule-row');
    if (!row) return;
    const idx = parseInt(row.dataset.idx);
    if (e.target.classList.contains('rule-type')) {
      rules[idx].rule_type = e.target.value;
      if (e.target.value !== 'continent') rules[idx].continent = '';
      if (e.target.value !== 'china') { rules[idx].isp = ''; rules[idx].province = ''; }
      renderRules();
    } else if (e.target.classList.contains('continent')) {
      rules[idx].continent = e.target.value;
    } else if (e.target.classList.contains('china-isp')) {
      rules[idx].isp = e.target.value;
    } else if (e.target.classList.contains('record-ttl')) {
      rules[idx].ttl = parseInt(e.target.value);
    }
  });
  document.getElementById('rules-container').addEventListener('input', (e) => {
    const row = e.target.closest('.rule-row');
    if (!row) return;
    const idx = parseInt(row.dataset.idx);
    if (e.target.classList.contains('record-value')) rules[idx].value = e.target.value;
    else if (e.target.classList.contains('china-province')) rules[idx].province = e.target.value;
  });
  document.getElementById('rules-container').addEventListener('click', (e) => {
    if (e.target.classList.contains('delete-rule')) {
      const row = e.target.closest('.rule-row');
      rules.splice(parseInt(row.dataset.idx), 1);
      renderRules();
    }
  });
}
