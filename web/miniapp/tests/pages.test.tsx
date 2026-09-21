import { beforeEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from '../src/app/App';
import { DraftProvider } from '../src/features/issues/drafts';
import { SessionProvider } from '../src/features/session/SessionProvider';
import { api } from '../src/shared/api';
import { ApiError } from '../src/shared/api/client';
import { context, issue, ids, statement, attachment } from './fixtures';
import { formatDate, categoryLabels, statusLabels } from '../src/shared/utils/presentation';
function mount(path: string) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <DraftProvider>
          <SessionProvider>
            <App />
          </SessionProvider>
        </DraftProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return qc;
}
beforeEach(() => {
  window.WebApp = { initData: 'test-only' };
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  vi.stubGlobal('URL', URL);
  URL.createObjectURL = vi.fn(() => 'blob:test-only');
  URL.revokeObjectURL = vi.fn();
  vi.spyOn(api, 'bootstrap').mockResolvedValue({
    user_context: context,
    expires_at: '2099-01-01T00:00:00Z',
  });
  vi.spyOn(api, 'issues').mockResolvedValue({ items: [], next_page_token: '' });
  vi.spyOn(api, 'announcements').mockResolvedValue({ items: [], next_page_token: '' });
  vi.spyOn(api, 'issue').mockResolvedValue({
    issue,
    attachments: [],
    confirmed_by_me: false,
    timeline: [],
    latest_statement: null,
  });
});
describe('resident and chairman components', () => {
  it('labels contract enums and missing dates', () => {
    expect(categoryLabels.SAFETY).toBe('Безопасность');
    expect(statusLabels.WAITING_RESULT).toBe('Ожидает результата');
    expect(formatDate(null)).toBe('—');
  });
  it('requires an ACTIVE membership', async () => {
    vi.mocked(api.bootstrap).mockResolvedValue({
      user_context: {
        ...context,
        memberships: context.memberships.map((m) => ({ ...m, status: 'INACTIVE' })),
      },
      expires_at: '2099-01-01T00:00:00Z',
    });
    mount('/chairman');
    expect(await screen.findByRole('heading', { name: 'Нет доступа к дому' })).toBeVisible();
    expect(api.issues).not.toHaveBeenCalled();
  });
  it('never enables submit for text without a photo and consent', async () => {
    mount('/issues/new');
    await screen.findByLabelText(/Что случилось/);
    await userEvent.type(screen.getByLabelText(/Что случилось/), 'Проблема');
    expect(screen.getByRole('button', { name: 'Отправить проблему' })).toBeDisabled();
  });
  it('retains entered text and never creates Issue after complete fails', async () => {
    vi.spyOn(api, 'createUpload').mockResolvedValue({
      upload_id: ids.attachment,
      presigned_url: 'https://storage.yandexcloud.net/test',
      expires_at: '2099-01-01T00:00:00Z',
      required_headers: {},
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 200 })));
    vi.spyOn(api, 'completeUpload').mockRejectedValue(new ApiError(503, 'DEPENDENCY_UNAVAILABLE'));
    const create = vi.spyOn(api, 'createIssue');
    mount('/issues/new');
    await userEvent.type(await screen.findByLabelText(/Что случилось/), 'Мусор');
    await userEvent.upload(
      screen.getByLabelText(/Добавить фотографии/),
      new File(['png'], 'photo.png', { type: 'image/png' }),
    );
    await userEvent.click(screen.getByRole('checkbox'));
    await userEvent.click(screen.getByRole('button', { name: 'Отправить проблему' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервис временно недоступен');
    expect(screen.getByLabelText(/Что случилось/)).toHaveValue('Мусор');
    expect(create).not.toHaveBeenCalled();
  });
  it('renders attachment from server signed URL and statement as text', async () => {
    vi.mocked(api.bootstrap).mockResolvedValue({
      user_context: {
        ...context,
        memberships: context.memberships.map((m) => ({ ...m, role: 'CHAIRMAN' })),
      },
      expires_at: '2099-01-01T00:00:00Z',
    });
    vi.mocked(api.issue).mockResolvedValue({
      issue,
      attachments: [attachment],
      confirmed_by_me: false,
      timeline: [],
      latest_statement: statement,
    });
    vi.spyOn(api, 'download').mockResolvedValue({
      url: 'https://storage.yandexcloud.net/photo',
      expires_at: '2099-01-01T00:00:00Z',
    });
    vi.spyOn(api, 'statement').mockResolvedValue({
      ...statement,
      body: '<script>unsafe()</script>',
    });
    mount('/chairman/issues/' + ids.issue);
    expect(await screen.findByAltText('photo.png')).toHaveAttribute(
      'src',
      'https://storage.yandexcloud.net/photo',
    );
    expect(await screen.findByText('<script>unsafe()</script>')).toBeVisible();
    expect(document.querySelector('script')).toBeNull();
  });
  it('shows a permission error without privileged actions', async () => {
    vi.mocked(api.issue).mockRejectedValue(new ApiError(403, 'PERMISSION_DENIED'));
    mount('/issues/' + ids.issue);
    expect(await screen.findByRole('alert')).toHaveTextContent('нет доступа');
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: 'Сформировать заявление' }),
      ).not.toBeInTheDocument(),
    );
  });
});
