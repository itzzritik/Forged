#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');

const pkgDir = path.join(__dirname, '..');
const pkg = require('../package.json');
const exe = process.platform === 'win32' ? '.exe' : '';
const binaries = ['forged', 'forged-sign', 'forged-auth'].map((name) => name + exe);

function fail(message) {
	console.error(message);
	process.exit(1);
}

function cacheRoot() {
	const home = os.homedir();
	if (process.platform === 'win32') {
		return path.join(process.env.LOCALAPPDATA || path.join(home, 'AppData', 'Local'), 'forged', 'cache');
	}
	if (process.platform === 'darwin') return path.join(home, 'Library', 'Caches', 'forged');
	const xdg = process.env.XDG_CACHE_HOME;
	return path.join(xdg && path.isAbsolute(xdg) ? xdg : path.join(home, '.cache'), 'forged');
}

function ready(dir) {
	try {
		return (
			fs.readFileSync(path.join(dir, '.version'), 'utf8').trim() === pkg.version &&
			binaries.every((name) => fs.existsSync(path.join(dir, name)))
		);
	} catch {
		return false;
	}
}

function writable(dir) {
	try {
		fs.mkdirSync(dir, { recursive: true });
		const probe = path.join(dir, `.probe-${process.pid}`);
		fs.writeFileSync(probe, '');
		fs.rmSync(probe);
		return true;
	} catch {
		return false;
	}
}

function download(url, dest, redirects = 5) {
	return new Promise((resolve, reject) => {
		const req = https.get(url, { headers: { 'User-Agent': `forgedssh/${pkg.version}` } }, (res) => {
			if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location && redirects > 0) {
				res.resume();
				resolve(download(new URL(res.headers.location, url).toString(), dest, redirects - 1));
				return;
			}
			if (res.statusCode !== 200) {
				res.resume();
				reject(new Error(`HTTP ${res.statusCode}`));
				return;
			}
			const file = fs.createWriteStream(dest);
			res.on('error', reject);
			file.on('error', reject);
			file.on('finish', () => file.close(resolve));
			res.pipe(file);
		});
		req.setTimeout(30000, () => req.destroy(new Error('timed out')));
		req.on('error', reject);
	});
}

async function install(target, archive) {
	const tmp = fs.mkdtempSync(path.join(target, '.download-'));
	try {
		const file = path.join(tmp, path.basename(new URL(archive.url).pathname));
		try {
			await download(archive.url, file);
		} catch (err) {
			throw new Error(`could not download ${archive.url}: ${err.message}\nCheck your connection and run forged again.`);
		}
		if (createHash('sha256').update(fs.readFileSync(file)).digest('hex') !== archive.sha256) {
			throw new Error(`checksum mismatch for ${archive.url}`);
		}
		// System32 tar reads zip; a Git Bash GNU tar earlier on PATH cannot.
		const tar = process.platform === 'win32' ? path.join(process.env.SystemRoot || 'C:\\Windows', 'System32', 'tar.exe') : 'tar';
		// Run as root, tar would keep the archive's CI owner (uid 1001) on the binaries.
		const extracted = spawnSync(tar, [process.platform === 'win32' ? '-xf' : '-xzf', file, '--no-same-owner', '-C', tmp], {
			stdio: 'ignore',
		});
		if (extracted.status !== 0) throw new Error(`could not extract ${archive.url}`);
		for (const name of binaries) {
			const from = path.join(tmp, name);
			if (!fs.existsSync(from)) throw new Error(`${name} is missing from ${archive.url}`);
			fs.chmodSync(from, 0o755);
			fs.renameSync(from, path.join(target, name));
		}
		fs.writeFileSync(path.join(tmp, '.version'), pkg.version);
		fs.renameSync(path.join(tmp, '.version'), path.join(target, '.version'));
	} finally {
		fs.rmSync(tmp, { recursive: true, force: true });
	}
}

function pruneCache(keep) {
	try {
		for (const entry of fs.readdirSync(cacheRoot())) {
			if (entry !== keep) fs.rmSync(path.join(cacheRoot(), entry), { recursive: true, force: true });
		}
	} catch {}
}

async function main() {
	const archives = pkg.forged && pkg.forged.archives;
	if (!archives) fail('forged: this package has no release binaries');
	const archive = archives[`${process.platform}-${process.arch}`];
	if (!archive) fail(`Error: unsupported platform "${process.platform}" (${process.arch})`);

	const dirs = [path.join(pkgDir, 'bin'), path.join(cacheRoot(), pkg.version)];
	let dir = dirs.find(ready);
	if (!dir) {
		dir = writable(dirs[0]) ? dirs[0] : dirs[1];
		fs.mkdirSync(dir, { recursive: true });
		process.stderr.write(`forged: downloading v${pkg.version}...\n`);
		try {
			await install(dir, archive);
		} catch (err) {
			// A launch running at the same time may have finished the same install.
			if (!ready(dir)) fail(`forged: ${err.message}`);
		}
		if (dir === dirs[1]) pruneCache(pkg.version);
	}

	const result = spawnSync(path.join(dir, `forged${exe}`), process.argv.slice(2), {
		stdio: 'inherit',
		env: { ...process.env, FORGED_NPM_PACKAGE_DIR: pkgDir },
	});
	if (result.error) fail(`Error: failed to execute forged binary: ${result.error.message}`);
	process.exit(result.status ?? 1);
}

main();
