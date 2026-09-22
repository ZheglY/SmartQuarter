/* global process, console */
// Run the real isolated backend acceptance stack from the Mini App package.
import { spawnSync } from 'node:child_process';
import { fileURLToPath, URL } from 'node:url';
const root = fileURLToPath(new URL('../../../', import.meta.url));
const options = {
  cwd: root,
  windowsHide: true,
  stdio: 'inherit',
  env: { ...process.env, COMPOSE_PARALLEL_LIMIT: process.env.COMPOSE_PARALLEL_LIMIT || '1' },
};
const compose = ['compose', '-f', 'deploy/test/compose.yaml'];
const test = spawnSync('docker', [...compose, 'run', '--build', '--rm', 'test'], {
  ...options,
  timeout: 25 * 60 * 1000,
});
if (test.error) console.error(test.error.message);
process.exitCode = test.status ?? 1;
// This Compose project contains only synthetic test data, never server volumes.
const cleanup = spawnSync('docker', [...compose, 'down', '-v'], {
  ...options,
  timeout: 60 * 1000,
});
if (cleanup.error) console.error(cleanup.error.message);
if (cleanup.status !== 0) process.exitCode ||= cleanup.status ?? 1;
