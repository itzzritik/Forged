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

const platformPackages = PLATFORM_MAP[process.platform];
const pkg = platformPackages && platformPackages[process.arch];
if (!pkg) {
	process.exit(0);
}

let pkgDir;
try {
	pkgDir = dirname(require.resolve(`${pkg}/package.json`));
} catch {
	process.exit(0);
}

const ext = process.platform === 'win32' ? '.exe' : '';
const bin = join(pkgDir, 'bin', `forged${ext}`);

spawnSync(bin, ['__daemon-freshen', '--quiet'], {
	stdio: 'ignore',
	timeout: 15000,
	windowsHide: true,
});

process.exit(0);
