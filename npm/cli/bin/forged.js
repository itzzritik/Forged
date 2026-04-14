#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const { join, dirname } = require('node:path');

const PLATFORM_MAP = {
	darwin: {
		x64: '@getforged/cli-darwin-x64',
		arm64: '@getforged/cli-darwin-arm64',
	},
	linux: {
		x64: '@getforged/cli-linux-x64',
		arm64: '@getforged/cli-linux-arm64',
	},
	win32: {
		x64: '@getforged/cli-win32-x64',
		arm64: '@getforged/cli-win32-arm64',
	},
};

const platform = process.platform;
const arch = process.arch;

const platformPackages = PLATFORM_MAP[platform];
if (!platformPackages) {
	console.error(`Error: unsupported platform "${platform}"`);
	process.exit(1);
}

const pkg = platformPackages[arch];
if (!pkg) {
	console.error(`Error: unsupported architecture "${arch}" on platform "${platform}"`);
	process.exit(1);
}

let pkgDir;
try {
	pkgDir = dirname(require.resolve(`${pkg}/package.json`));
} catch {
	console.error(`Error: required package "${pkg}" is not installed`);
	console.error(`Please reinstall with: npm install -g @getforged/cli`);
	process.exit(1);
}

const ext = platform === 'win32' ? '.exe' : '';
const bin = join(pkgDir, 'bin', `forged${ext}`);

const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
	console.error(`Error: failed to execute forged binary: ${result.error.message}`);
	process.exit(1);
}

process.exit(result.status ?? 1);
