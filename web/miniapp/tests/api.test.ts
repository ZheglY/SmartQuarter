import { describe, it, expect, vi } from 'vitest';
import { z } from 'zod';
import { request, ApiError, setUnauthorizedHandler, uncertain } from '../src/shared/api/client';
import { api } from '../src/shared/api';
import { context, issue } from './fixtures';
const json = (v: unknown, status = 200) =>
  new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } });
describe('REST boundary', () => {
  it('sends cookies and raw initData, never actor claims', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(json({ user_context: context, expires_at: '2099-01-01T00:00:00Z' }));
    vi.stubGlobal('fetch', fetcher);
    await api.bootstrap('signed=raw%2Bvalue');
    expect(fetcher.mock.calls[0][0]).toBe('/api/v1/session/max');
    const options = fetcher.mock.calls[0][1];
    expect(options.credentials).toBe('include');
    expect(JSON.parse(options.body)).toEqual({ init_data: 'signed=raw%2Bvalue' });
    expect(Object.keys(options.headers).join()).not.toMatch(/Authorization|X-User|X-House|X-Role/);
  });
  it('rejects malformed successful DTOs', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json({ items: 'invalid' })));
    await expect(api.issues()).rejects.toMatchObject({ code: 'INVALID_RESPONSE' });
  });
  it('clears session on protected 401 but not failed bootstrap', async () => {
    const handler = vi.fn();
    const dispose = setUnauthorizedHandler(handler);
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation(() =>
          Promise.resolve(json({ error: { code: 'UNAUTHENTICATED' } }, 401)),
        ),
    );
    await expect(api.me()).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledTimes(1);
    await expect(api.bootstrap('bad')).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledTimes(1);
    dispose();
  });
  it('preserves idempotency and does not retry an ambiguous mutation', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('sensitive upstream URL'));
    vi.stubGlobal('fetch', fetcher);
    const body = {
      category: issue.category,
      description: issue.description,
      location_text: '',
      attachment_ids: [],
    };
    await expect(api.createIssue(body, 'stable-key')).rejects.toMatchObject({ status: 0 });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher.mock.calls[0][1].headers['Idempotency-Key']).toBe('stable-key');
  });
  it('encodes pagination and repeated status filters', async () => {
    const f = vi.fn().mockResolvedValue(json({ items: [], next_page_token: '' }));
    vi.stubGlobal('fetch', f);
    await api.issues(true, 'a+/=', ['DETECTED', 'CONFIRMING']);
    expect(f.mock.calls[0][0]).toContain('page_token=a%2B%2F%3D&status=DETECTED&status=CONFIRMING');
  });
  it('does not display raw server error or initData', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        json(
          {
            error: {
              code: 'DEPENDENCY_UNAVAILABLE',
              message: 'secret token',
              request_id: 'support-id',
            },
          },
          503,
        ),
      ),
    );
    try {
      await api.me();
      throw new Error('unexpected success');
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).message).not.toContain('secret');
      expect((e as ApiError).requestId).toBe('support-id');
    }
  });
  it('handles 204 and cancellation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    await expect(api.logout()).resolves.toBeUndefined();
    const c = new AbortController();
    c.abort();
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new DOMException('Aborted', 'AbortError')));
    await expect(request('/me', z.unknown(), { signal: c.signal })).rejects.toMatchObject({
      name: 'AbortError',
    });
  });
  it('classifies ambiguous errors', () => {
    expect(uncertain(new ApiError(504, 'TIMEOUT'))).toBe(true);
    expect(uncertain(new ApiError(409, 'IDEMPOTENCY_IN_PROGRESS'))).toBe(true);
    expect(uncertain(new ApiError(400, 'INVALID_ARGUMENT'))).toBe(false);
  });
});
