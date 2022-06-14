(() => {
    let loginingIn = false
    async function handleLogin() {
        if (loginingIn) {
            return;
        }
        loginingIn = true;
        try {
            let username = document.getElementById("username").value;
            let password = document.getElementById("password").value;

            if (!username || !password) {
                showInfo("Missing username and password");
                return
            }
            let hdrs = new Headers();
            hdrs.append('Authorization', `Basic ${btoa(username + ':' + password)}`);
            let res = await fetch("/.auth/.login", {
                method: "POST",
                headers: hdrs
            }).then((res) => {
                if (res.status != 200) {
                    throw new Error("User/Password combination is not valid");
                }
                return res.json();
            });

            if (!res.token) {
                throw new Error("Credentials accepted but server did not sent a valid token");
            }

            localStorage.setItem('/.auth/.login', JSON.stringify(res));
            showInfo("Welcome!");
            location.reload();

        } catch(err) {
            showInfo(`Unable to perform login: ${err}`);
        } finally {
            loginingIn = false;
        }
    }
    function showInfo(msg) {
        let box = document.getElementById("info-box");
        if (!box) {
            console.info('info-box', msg);
        }
        box.innerHTML = `<strong>${msg}</strong>`
    }
    async function validateToken() {
        let item = localStorage.getItem('/.auth/.login')
        if (!item) {
            return;
        }
        let lastLogin = JSON.parse(item);
        let hdrs = new Headers();
        hdrs.append('Authorization', `Bearer ${lastLogin.token}`);

        let res = await fetch('/.auth/.health', {headers: hdrs, method: 'GET'});
        if (res.status != 200) {
            showInfo('Login has expired, you might need to login again!');
            localStorage.removeItem('/.auth/.login');
            return;
        }
        showInfo('Last login is still valid, you can start using the admin panel!');
    }
    window.onload = async () => {
        await validateToken();
        document.getElementById("login").onclick = (ev) => {
            ev.preventDefault();
            handleLogin();
        }
    }
})();
