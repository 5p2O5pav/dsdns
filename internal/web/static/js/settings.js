async function loadTelegramSettings() {
  const cfg = await api('/settings/telegram');
  document.getElementById('tg-enabled').checked = cfg.enabled;
  document.getElementById('tg-bot-token').value = cfg.bot_token || '';
  document.getElementById('tg-chat-id').value = cfg.chat_id || '';
}

async function saveTelegramSettings() {
  const enabled = document.getElementById('tg-enabled').checked;
  const botToken = document.getElementById('tg-bot-token').value.trim();
  const chatId = document.getElementById('tg-chat-id').value.trim();
  if (!botToken || !chatId) {
    alert('请填写完整的 Bot Token 和 Chat ID');
    return;
  }
  await api('/settings/telegram', {
    method: 'PUT',
    body: JSON.stringify({ enabled, bot_token: botToken, chat_id: chatId })
  });
  alert('设置已保存');
}

function initSettings() {
  document.getElementById('module-settings').innerHTML = `
    <div class="card">
      <div class="card-header"><h2>Telegram 告警设置</h2></div>
      <div style="padding:1.5rem; display:flex; flex-direction:column; gap:1rem; max-width:400px;">
        <label><input type="checkbox" id="tg-enabled"> 启用告警</label>
        <label>Bot Token</label>
        <input id="tg-bot-token" placeholder="123456:ABC-DEF...">
        <label>Chat ID</label>
        <input id="tg-chat-id" placeholder="123456789">
        <button id="save-telegram" class="btn btn-primary">保存</button>
      </div>
    </div>
  `;
  loadTelegramSettings();
  document.getElementById('save-telegram').onclick = saveTelegramSettings;
}
