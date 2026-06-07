const API_BASE = '/api';

async function api(url, options = {}) {
  const token = localStorage.getItem('token');
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  if (token) {
    headers['Authorization'] = 'Bearer ' + token;
  }
  const res = await fetch(API_BASE + url, {
    ...options,
    headers,
  });
  if (res.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/static/login.html';
    return;
  }
  const json = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(json.message || `HTTP ${res.status}`);
  }
  // 统一返回 data 字段（如果存在），否则返回整个 json
  return json.data !== undefined ? json.data : json;
}
