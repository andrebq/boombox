window.onload = () => {
    let processing = false;
    async function callEnableRouteAPI(asset, route) {
        let hdrs = new Headers();
        await addAuthHeaders(hdrs);
        let res = await fetch(`/.auth/.internals/set-route`, {
            method: 'POST',
            body: JSON.stringify({
                route: route,
                asset: asset,
                methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS', 'HEAD']
            }),
            headers: hdrs,
        });
        if (res.status != 200) {
            throw new Error(`Invalid code for set route: ${res.status}`);
        }
    }
    async function enableRoute() {
        if (processing) {
            showStatus('Processing previous request, please wait a few seconds and try again later...');
        }
        try {
            let asset = document.getElementById('asset-path').value;
            let route = document.getElementById('route').value;

            if (!asset) {
                showStatus('Cannot enable a new route without a valid codebase, please verify');
                return
            }
            if (!route) {
                showStatus('Cannot enable a new route without a valid route, please verify');
                return;
            }
            await callEnableRouteAPI(asset, route);
            showStatus('Route updated!');
        } finally {
            processing = false;
        }
    }

    document.getElementById('upload').onclick = async (ev) => {
        await enableRoute();
    }


    function showStatus(statusMsg) {
        document.getElementById('info-box').innerHTML = `<strong>${statusMsg}</strong>`;
    }
}
