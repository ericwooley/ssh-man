#!/usr/bin/env node

'use strict';

const { accessSync, constants, statSync } = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const rootDir = path.resolve(__dirname, '..');
const frontendDir = path.join(rootDir, 'frontend');
const forwardedArguments = process.argv.slice(2);

function resolveCommand(command) {
  const pathEntries = (process.env.PATH || '')
    .split(path.delimiter)
    .filter(Boolean)
    .map((entry) => entry.replace(/^"(.*)"$/, '$1'));

  const extensions =
    process.platform === 'win32'
      ? (process.env.PATHEXT || '.COM;.EXE;.BAT;.CMD').split(';')
      : [''];

  for (const directory of pathEntries) {
    for (const extension of extensions) {
      const candidate = path.join(directory, `${command}${extension}`);

      try {
        accessSync(candidate, constants.X_OK);
        if (statSync(candidate).isFile()) {
          return candidate;
        }
      } catch {
        // Keep searching PATH.
      }
    }
  }

  return null;
}

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: frontendDir,
    shell: false,
    stdio: 'inherit',
  });

  if (result.error) {
    console.error(`Unable to run ${path.basename(command)}: ${result.error.message}`);
    return 1;
  }

  return result.status === null ? 1 : result.status;
}

function resolveNodeShimEntrypoint(command, relativeEntrypoint) {
  if (process.platform !== 'win32' || !/\.(?:bat|cmd)$/i.test(command)) {
    return null;
  }

  const candidate = path.join(path.dirname(command), ...relativeEntrypoint);
  try {
    if (statSync(candidate).isFile()) {
      return candidate;
    }
  } catch {
    // The shim is not backed by the usual Node package layout.
  }
  return null;
}

const corepack = resolveCommand('corepack');
if (corepack) {
  const entrypoint = resolveNodeShimEntrypoint(
    corepack,
    ['node_modules', 'corepack', 'dist', 'corepack.js'],
  );
  if (entrypoint) {
    process.exit(run(process.execPath, [entrypoint, 'pnpm', ...forwardedArguments]));
  }
  if (!/\.(?:bat|cmd)$/i.test(corepack)) {
    process.exit(run(corepack, ['pnpm', ...forwardedArguments]));
  }
}

const pnpm = resolveCommand('pnpm');
if (pnpm) {
  const entrypoint = resolveNodeShimEntrypoint(
    pnpm,
    ['node_modules', 'pnpm', 'bin', 'pnpm.mjs'],
  );
  if (entrypoint) {
    process.exit(run(process.execPath, [entrypoint, ...forwardedArguments]));
  }
  if (!/\.(?:bat|cmd)$/i.test(pnpm)) {
    process.exit(run(pnpm, forwardedArguments));
  }
}

console.error(
  'Missing required command: install Corepack or pnpm 11.17.0 and ensure it is on PATH.',
);
process.exit(127);
