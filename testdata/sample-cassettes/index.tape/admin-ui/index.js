(() => {
    let loginingIn = false
    async function handleLogin() {
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
    window.onload = () => {
        document.getElementById("login").onclick = (ev) => {
            ev.preventDefault();
            handleLogin();
        }
    }
})();
