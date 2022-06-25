(() => {
    let loadingFile = false;
    let uploading = false;
    let content = '';
    let targetPath = '';

    async function callUploadAPI(content, path) {
        let hdrs = new Headers();
        await addAuthHeaders(hdrs);
        let res = await fetch(`/.auth/.internals/write-asset/${encodeURIComponent(path)}`, {
            method: 'PUT',
            body: content,
            headers: hdrs,
        });
        if (res.status != 200) {
            throw new Error(`Invalid code for upload: ${res.status}`);
        }
        res = await fetch(`/.auth/.internals/enable-code/${encodeURIComponent(path)}?enabled=true`, {
            method: 'PUT',
            body: JSON.stringify({}),
            headers: hdrs,
        });
        if (res.status != 200) {
            throw new Error(`Unable to configure [${path}] as codebase, got status code: ${res.status}`);
        }
        return true;
    }
    async function uploadCode() {
        targetPath = document.getElementById('asset-path').value;
        content = document.getElementById('code').value;
        if (uploading) {
            showStatus('An upload is in progress, please wait a bit...');
            return;
        }
        if (!targetPath) {
            showStatus('Please inform the desired path and try again');
            return;
        }
        if (!content) {
            showStatus('Please provide a valid lua code code and try again');
            return;
        }
        uploading = true;
        try {
            showStatus('Uploading file...');
            try {
                await callUploadAPI(content, targetPath);
                showStatus(`New code uploaded to ${targetPath}`);
            } catch(err) {
                console.error('Upload error', err)
                showStatus(`Unable to finish previous upload: ${err}`)
            }
        } finally {
            uploading = false;
        }
    }

    function showStatus(statusMsg) {
        document.getElementById('info-box').innerHTML = `<strong>${statusMsg}</strong>`;
    }

    function registerHandlers() {
        document.getElementById('code').onchange = (event) => {
            content = event.target.value;
        };
        document.getElementById('code').onblur = (event) => {
            content = event.target.value;
        }
        document.getElementById('upload').onclick = async (ev) => {
            ev.preventDefault();
            uploadCode();
        }
    }

    async function loadInitialValues() {
        content = document.getElementById('code').value;
        targetPath = document.getElementById('asset-path').value;
    }

    window.onload = () => {
        registerHandlers();
        loadInitialValues();
    }

})();
