import { describe, it, expect, vi } from 'vitest';
import { api } from '../src/shared/api';
import { context, ids, issue, attachment, statement, announcement } from './fixtures';
const details = {
  issue,
  attachments: [],
  confirmed_by_me: false,
  timeline: [],
  latest_statement: null,
};
describe('Gateway DTO and route contract', () => {
  const cases: [string, string, string, () => Promise<unknown>, unknown, unknown][] = [
    ['me', 'GET', '/me', () => api.me(), context, undefined],
    [
      'switch house',
      'POST',
      '/session/active-house',
      () => api.switchHouse(ids.house2),
      { active_house_id: ids.house2, role: 'RESIDENT' },
      { house_id: ids.house2 },
    ],
    [
      'create upload',
      'POST',
      '/uploads',
      () => api.createUpload({ filename: 'photo.png', mime_type: 'image/png', size_bytes: 68 }),
      {
        upload_id: ids.attachment,
        presigned_url: 'https://storage.yandexcloud.net/a',
        expires_at: '2099-01-01T00:00:00Z',
        required_headers: {},
      },
      { filename: 'photo.png', mime_type: 'image/png', size_bytes: 68 },
    ],
    [
      'complete upload',
      'POST',
      '/uploads/' + ids.attachment + '/complete',
      () => api.completeUpload(ids.attachment),
      attachment,
      {},
    ],
    [
      'download',
      'GET',
      '/attachments/' + ids.attachment + '/download-url',
      () => api.download(ids.attachment),
      { url: 'https://storage.yandexcloud.net/a', expires_at: '2099-01-01T00:00:00Z' },
      undefined,
    ],
    [
      'create issue',
      'POST',
      '/issues',
      () =>
        api.createIssue(
          {
            category: 'OTHER',
            description: 'Текст',
            location_text: '',
            attachment_ids: [ids.attachment],
          },
          'key',
        ),
      issue,
      {
        category: 'OTHER',
        description: 'Текст',
        location_text: '',
        attachment_ids: [ids.attachment],
      },
    ],
    ['details', 'GET', '/issues/' + ids.issue, () => api.issue(ids.issue), details, undefined],
    [
      'confirm',
      'POST',
      '/issues/' + ids.issue + '/confirm',
      () => api.confirm(ids.issue),
      { confirmation_count: 2, confirmed_by_me: true },
      {},
    ],
    [
      'get statement',
      'GET',
      '/issues/' + ids.issue + '/statement',
      () => api.statement(ids.issue),
      statement,
      undefined,
    ],
    [
      'generate',
      'POST',
      '/issues/' + ids.issue + '/statement',
      () => api.generate(ids.issue, 'Примечание', 'key'),
      statement,
      { chairman_note: 'Примечание' },
    ],
    [
      'status',
      'PATCH',
      '/issues/' + ids.issue + '/status',
      () => api.status(ids.issue, 'CONFIRMING'),
      { ...issue, status: 'CONFIRMING' },
      { new_status: 'CONFIRMING' },
    ],
    [
      'list announcements',
      'GET',
      '/announcements?page_size=20',
      () => api.announcements(),
      { items: [announcement], next_page_token: '' },
      undefined,
    ],
    [
      'create announcement',
      'POST',
      '/announcements',
      () => api.announce('Title', 'Body', 'key'),
      announcement,
      { title: 'Title', body: 'Body' },
    ],
  ];
  it.each(cases)('%s', async (_name, method, path, run, response, body) => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify(response), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );
    vi.stubGlobal('fetch', fetcher);
    await expect(run()).resolves.toEqual(response);
    expect(fetcher.mock.calls[0][0]).toBe('/api/v1' + path);
    expect(fetcher.mock.calls[0][1].method).toBe(method);
    expect(fetcher.mock.calls[0][1].body).toBe(
      body === undefined ? undefined : JSON.stringify(body),
    );
  });
});
