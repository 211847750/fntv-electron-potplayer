#!/usr/bin/env node

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const version = '2.1.25156';
const fileName = `winfsp-${version}.msi`;
const url = `https://github.com/winfsp/winfsp/releases/download/v2.1/${fileName}`;
const expectedSha256 = '073a70e00f77423e34bed98b86e600def93393ba5822204fac57a29324db9f7a';
const targetDir = path.join(__dirname, '..', 'third_party', 'winfsp');
const targetPath = path.join(targetDir, fileName);

async function download() {
    fs.mkdirSync(targetDir, { recursive: true });

    if (fs.existsSync(targetPath)) {
        const currentHash = await sha256File(targetPath);
        if (currentHash === expectedSha256) {
            console.log(`WinFsp installer already prepared: ${targetPath}`);
            return;
        }
        fs.rmSync(targetPath, { force: true });
    }

    console.log(`Downloading WinFsp ${version}...`);
    const response = await fetch(url, {
        headers: { 'User-Agent': 'fntv-electron-potplayer-build' },
        redirect: 'follow',
    });
    if (!response.ok || !response.body) {
        throw new Error(`Failed to download WinFsp: HTTP ${response.status}`);
    }

    const tempPath = `${targetPath}.tmp`;
    const file = fs.createWriteStream(tempPath);
    await new Promise((resolve, reject) => {
        const reader = response.body.getReader();

        function pump() {
            reader.read().then(({ done, value }) => {
                if (done) {
                    file.end(resolve);
                    return;
                }
                file.write(Buffer.from(value), error => {
                    if (error) {
                        reject(error);
                        return;
                    }
                    pump();
                });
            }).catch(reject);
        }

        file.on('error', reject);
        pump();
    });

    const actualHash = await sha256File(tempPath);
    if (actualHash !== expectedSha256) {
        fs.rmSync(tempPath, { force: true });
        throw new Error(`WinFsp checksum mismatch: expected ${expectedSha256}, got ${actualHash}`);
    }

    fs.renameSync(tempPath, targetPath);
    console.log(`WinFsp installer prepared: ${targetPath}`);
}

function sha256File(filePath) {
    return new Promise((resolve, reject) => {
        const hash = crypto.createHash('sha256');
        const stream = fs.createReadStream(filePath);
        stream.on('data', chunk => hash.update(chunk));
        stream.on('error', reject);
        stream.on('end', () => resolve(hash.digest('hex')));
    });
}

download().catch(error => {
    console.error(error);
    process.exit(1);
});
