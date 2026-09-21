/* global process, fetch, AbortSignal, setTimeout, console, URLSearchParams */
// Runs the repository Gateway binary with an isolated local Redis and synthetic test credentials.
// It verifies the current integration blocker, not a successful MAX login or business E2E.
import { spawn } from 'node:child_process';
import { createHmac, randomBytes } from 'node:crypto';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
const port = Number(process.env.PROBE_PORT || 18090),
  origin = 'http://127.0.0.1:5173',
  base = `http://127.0.0.1:${port}`;
const token = randomBytes(32).toString('hex');
const child = spawn(resolve('test-results/gateway-probe.exe'), [], {
  windowsHide: true,
  stdio: 'ignore',
  env: {
    ...process.env,
    APP_ENV: 'test',
    HTTP_ADDR: `127.0.0.1:${port}`,
    REDIS_ADDR: '127.0.0.1:16389',
    REDIS_PASSWORD: '',
    REDIS_DB: '0',
    MAX_BOT_TOKEN: token,
    MAX_BOT_API_BASE_URL: 'http://127.0.0.1:19999',
    MAX_WEBHOOK_SECRET: 'test_only_secret',
    MAX_BOT_USERNAME: 'test_only_bot',
    MAX_MINIAPP_URL: origin,
    TRUSTED_ORIGINS: origin,
    ISSUE_GRPC_ADDR: '127.0.0.1:19998',
    ISSUE_READY_URL: 'http://127.0.0.1:19997/readyz',
    COMMUNITY_GRPC_ADDR: '',
    AUTH_RATE_PER_MINUTE: '100',
    NOTIFICATION_STREAM: 'frontend-probe-empty',
    NOTIFICATION_CONSUMER_GROUP: 'frontend-probe',
  },
});
let startError;
child.on('error', (e) => {
  startError = e;
});
async function call(path, method = 'GET', body, requestOrigin = origin) {
  return fetch(base + path, {
    method,
    headers: { Origin: requestOrigin, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(5000),
  });
}
try {
  let ready = false;
  for (let i = 0; i < 40; i++) {
    if (startError) throw startError;
    try {
      ready = (await call('/livez')).status === 200;
      if (ready) break;
    } catch {
      /* Wait for the child HTTP listener. */
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  assert(ready, 'Gateway did not start');
  assert.equal((await call('/livez')).status, 200);
  console.log('PASS GET /livez -> 200');
  assert.equal((await call('/api/v1/me')).status, 401);
  console.log('PASS GET /api/v1/me without cookie -> 401');
  assert.equal((await call('/api/v1/session/max', 'POST', { init_data: 'invalid' })).status, 401);
  console.log('PASS invalid initData -> 401');
  assert.equal(
    (
      await call(
        '/api/v1/session/max',
        'POST',
        { init_data: 'invalid' },
        'https://untrusted.example',
      )
    ).status,
    403,
  );
  console.log('PASS untrusted Origin -> 403');
  for (let i = 1; i <= 5; i++) {
    const values = new URLSearchParams({
      auth_date: String(Math.floor(Date.now() / 1000)),
      user: JSON.stringify({ id: 100001, first_name: 'Probe' }),
    });
    const data = [...values.keys()]
      .sort()
      .map((k) => `${k}=${values.get(k)}`)
      .join('\n');
    const secret = createHmac('sha256', 'WebAppData').update(token).digest();
    values.set('hash', createHmac('sha256', secret).update(data).digest('hex'));
    const response = await call('/api/v1/session/max', 'POST', { init_data: values.toString() });
    assert.equal(response.status, 503);
    const result = await response.json();
    assert.equal(result.error.code, 'DEPENDENCY_UNAVAILABLE');
    assert.equal(response.headers.get('set-cookie'), null);
    console.log(
      `BLOCKED ${i}/5 POST /api/v1/session/max -> 503 DEPENDENCY_UNAVAILABLE; no session cookie`,
    );
  }
  assert.equal((await call('/readyz')).status, 503);
  console.log('PASS GET /readyz -> 503 (Identity unavailable; Issue also absent in probe)');
} finally {
  child.kill();
}
