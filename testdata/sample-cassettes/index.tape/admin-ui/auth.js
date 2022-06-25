async function addAuthHeaders(headers) {
    let item = localStorage.getItem('/.auth/.login')
    if (!item) {
        return;
    }
    let lastLogin = JSON.parse(item);
    headers.append('Authorization', `Bearer ${lastLogin.token}`);
}

