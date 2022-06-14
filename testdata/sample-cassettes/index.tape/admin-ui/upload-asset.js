(() => {
    let loadingFile = false;
    let uploading = false;
    let content = '';
    let targetPath = '';

    function readFile(blob) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = (event) => {
                resolve(event.target.result);
            };
            reader.onabort = reject;
            reader.onerror = reject;
            reader.readAsText(blob);
        });
    }
    async function callUploadAPI(content, path) {
        let res = await fetch(`/.auth/.internals/write-asset/${encodeURIComponent(path)}`, {
            method: 'PUT',
            body: content
        });
        if (res.status != 200) {
            throw new Error(`Invalid code for upload: ${res.status}`);
        }
        return true;
    }
    async function uploadFile() {
        if (uploading) {
            showStatus('An upload is in progress, please wait a bit...');
            return;
        }
        uploading = true;
        try {
        if (loadingFile) {
            showStatus('File is being loaded, please try agin in a couple of seconds...');
        } else if (!content) {
            showStatus('Please select a non-empty file and try again.');
        } else if (!targetPath) {
            showStatus('Please inform a valid asset path');
        }
        showStatus('Uploading file...');
            try {
                await callUploadAPI(content, targetPath);
            } catch(err) {
                console.error('Upload error', err)
                showStatus(`Unable to finish previous upload: ${err}`)
            }
        } finally {
            uploading = false;
        }
    }
    async function loadFile(fd) {
        content = '';
        loadingFile = true;
        try {
            content = await readFile(fd);
        } finally {
            loadingFile = false;
        }
    }

    function showStatus(statusMsg) {
        document.getElementById('info-box').innerHTML = `<strong>${statusMsg}</strong>`;
    }

    function registerHandlers() {
        document.getElementById('asset-file').onchange = async (event) => {
            if (!event.target.files) {
                return
            }
            loadFile(event.target.files[0]);
        };
        document.getElementById('upload').onclick = async (ev) => {
            ev.preventDefault();
            uploadFile();
        }
    }

    async function loadInitialValues() {
        let fileInput = document.getElementById('asset-file');
        if (fileInput.files) {
            await loadFile(fileInput.files[0]);
        }
        targetPath = document.getElementById('asset-path').value;
    }

    window.onload = () => {
        registerHandlers();
        loadInitialValues();
    }

})();
