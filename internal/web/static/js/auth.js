function getUserInfo() {
  const token = localStorage.getItem('token');
  if (!token) return null;
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch {
    return null;
  }
}

function isAdmin() {
  const info = getUserInfo();
  return info ? info.adm === true : false;
}
