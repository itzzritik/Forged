#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const readline = require('node:readline');

const pkgDir = path.join(__dirname, '..');
const pkg = require('../package.json');
const win32 = process.platform === 'win32';
const exe = win32 ? '.exe' : '';
const binaries = ['forged', 'forged-sign', 'forged-auth'].map((name) => name + exe);
// Must match cmd.ExitCode in the Go CLI.
const UPDATE_EXIT = 85;
const REGISTRY = 'https://registry.npmjs.org/forgedssh/latest';

function fail(message) {
	console.error(message);
	process.exit(1);
}

function cacheRoot() {
	const home = os.homedir();
	if (win32) {
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

function getJSON(url) {
	return new Promise((resolve, reject) => {
		const req = https.get(url, { headers: { 'User-Agent': `forgedssh/${pkg.version}`, Accept: 'application/json' } }, (res) => {
			if (res.statusCode !== 200) {
				res.resume();
				reject(new Error(`HTTP ${res.statusCode}`));
				return;
			}
			let body = '';
			res.setEncoding('utf8');
			res.on('data', (chunk) => {
				body += chunk;
				if (body.length > 1 << 20) req.destroy(new Error('reply too large'));
			});
			res.on('end', () => {
				try {
					resolve(JSON.parse(body));
				} catch (err) {
					reject(err);
				}
			});
			res.on('error', reject);
		});
		req.setTimeout(10000, () => req.destroy(new Error('timed out')));
		req.on('error', reject);
	});
}

function parseVersion(version) {
	const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(typeof version === 'string' ? version : '');
	return m && m.slice(1).map(Number);
}

function newer(latest, current) {
	const l = parseVersion(latest);
	const c = parseVersion(current);
	if (!l || !c) return false;
	const i = l.findIndex((n, k) => n !== c[k]);
	return i >= 0 && l[i] > c[i];
}

// Mirrors forged's own guard: real root (HOME=/root) may update, sudo from a user's shell may not.
function sudoInUserHome() {
	try {
		return process.getuid?.() === 0 && fs.statSync(os.homedir()).uid !== 0;
	} catch {
		return false;
	}
}

function confirm(question) {
	return new Promise((resolve) => {
		const rl = readline.createInterface({ input: process.stdin, output: process.stderr });
		rl.question(question, (answer) => {
			rl.close();
			resolve(/^y(es)?$/i.test(answer.trim()));
		});
	});
}

// Runs here, not in forged: Windows can't replace a folder whose exe is running, and npm must replace this one.
async function update(reopen) {
	if (sudoInUserHome()) {
		console.error('Run forged update as your own user, not with sudo');
		return 1;
	}
	let latest;
	try {
		latest = (await getJSON(REGISTRY)).version;
		if (!parseVersion(latest)) throw new Error('the npm registry sent an invalid version');
	} catch (err) {
		console.error(`forged: couldn't check for updates: ${err.message}`);
		return 1;
	}
	if (!newer(latest, pkg.version)) {
		console.log(`forged is up to date (${pkg.version})`);
		return 0;
	}
	if (pkgDir.split(path.sep).includes('@getforged')) {
		console.error('Move to the new package: npm rm -g @getforged/cli && npm i -g forgedssh');
		return 1;
	}
	// The npm beside this Node, run as a script: no PATH lookup or .cmd shim, and the same under sudo.
	let npm = '';
	let root = '';
	try {
		npm = fs.realpathSync(path.join(path.dirname(process.execPath), ...(win32 ? ['node_modules', 'npm', 'bin', 'npm-cli.js'] : ['npm'])));
		const res = spawnSync(process.execPath, [npm, 'root', '-g'], { encoding: 'utf8' });
		if (res.status === 0 && res.stdout.trim()) root = fs.realpathSync(res.stdout.trim());
	} catch {}
	if (!root || path.relative(root, pkgDir) !== 'forgedssh') {
		console.error('Update with the package manager you installed forged with (npm: npm i -g forgedssh@latest)');
		return 1;
	}
	const args = [npm, 'install', '-g', 'forgedssh@latest'];
	process.stderr.write(`forged: updating to v${latest}...\n`);
	const first = spawnSync(process.execPath, args, { stdio: ['inherit', 'inherit', 'pipe'], encoding: 'utf8', maxBuffer: 16 << 20 });
	let code = first.status;
	const denied = code !== 0 && !win32 && /EACCES|EPERM/.test(first.stderr || '');
	// A permission error gets the sudo offer instead of npm's wall of text.
	if (code !== 0 && !denied) process.stderr.write(first.stderr || '');
	if (denied && process.stdin.isTTY && (await confirm('npm needs admin rights. Run it with sudo? [y/N] '))) {
		code = spawnSync('sudo', ['-H', process.execPath, ...args], { stdio: 'inherit' }).status;
	}
	if (code !== 0) {
		console.error(`Update failed. Run it yourself: ${denied ? 'sudo ' : ''}npm i -g forgedssh@latest`);
		return 1;
	}
	const fresh = spawnSync(process.execPath, [path.join(pkgDir, 'bin', 'forged.js'), '__daemon-freshen', '--quiet'], { stdio: 'inherit' });
	if (fresh.status !== 0) {
		console.error(`Updated to ${latest}, but forged couldn't restart itself. Run forged to finish.`);
		return 1;
	}
	console.log(`Updated forged ${pkg.version} → ${latest}.${reopen ? '' : ' Run forged to unlock again.'}`);
	return 0;
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
		const tar = win32 ? path.join(process.env.SystemRoot || 'C:\\Windows', 'System32', 'tar.exe') : 'tar';
		// Run as root, tar would keep the archive's CI owner (uid 1001) on the binaries.
		const extracted = spawnSync(tar, [win32 ? '-xf' : '-xzf', file, '--no-same-owner', '-C', tmp], {
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
	if (process.argv.length === 3 && process.argv[2] === 'update') process.exit(await update(false));
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
	if (result.status === UPDATE_EXIT) {
		const code = await update(true);
		if (code !== 0) process.exit(code);
		process.exit(spawnSync(process.execPath, [path.join(pkgDir, 'bin', 'forged.js')], { stdio: 'inherit' }).status ?? 1);
	}
	process.exit(result.status ?? 1);
}

main();
