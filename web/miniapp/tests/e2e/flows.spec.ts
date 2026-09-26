import { test, expect, type Page } from '@playwright/test';
import { context, issue, attachment, statement, announcement, ids } from '../fixtures';
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aY9sAAAAASUVORK5CYII=',
  'base64',
);
async function setup(
  page: Page,
  role: 'RESIDENT' | 'CHAIRMAN' = 'RESIDENT',
  options: { expired?: boolean; ambiguous?: boolean } = {},
) {
  await page.route('https://st.max.ru/**', (r) => r.abort());
  await page.addInitScript(() => {
    window.WebApp = {
      initData: 'fixture-signed-data',
      platform: 'android',
      BackButton: { show() {}, hide() {}, onClick() {}, offClick() {} },
    };
  });
  let active = ids.house,
    confirmed = false,
    created = false,
    hasStatement = false,
    sent = false,
    attempts = 0;
  const keys: string[] = [];
  let currentIssue = { ...issue };
  await page.route('https://storage.yandexcloud.net/**', async (r) => {
    if (r.request().method() === 'PUT') {
      expect(r.request().headers()['cookie']).toBeUndefined();
      expect(r.request().headers()['authorization']).toBeUndefined();
      expect(r.request().headers()['x-amz-meta-test']).toBe('signed');
      await r.fulfill({ status: 200 });
    } else await r.fulfill({ contentType: 'image/png', body: png });
  });
  await page.route('**/api/v1/**', async (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname,
      method = req.method();
    const reply = (body: unknown, status = 200) => r.fulfill({ status, json: body });
    const userContext = {
      ...context,
      active_house_id: active,
      memberships: context.memberships.map((m) => ({ ...m, role })),
    };
    if (path.endsWith('/session/max'))
      return reply({ user_context: userContext, expires_at: '2099-01-01T00:00:00Z' });
    if (path.endsWith('/me')) return reply(userContext);
    if (path.endsWith('/house-access')) return reply({platform_admin:false,can_register_house:role==='CHAIRMAN',can_manage_active_house:role==='CHAIRMAN',pending_registrations:0,pending_join_requests:0,incoming_join_requests:0});
    if (path.endsWith('/session/active-house')) {
      active = req.postDataJSON().house_id;
      return reply({ active_house_id: active, role });
    }
    if (path.endsWith('/session/logout')) return r.fulfill({ status: 204 });
    if (path.endsWith('/uploads') && method === 'POST')
      return reply(
        {
          upload_id: ids.attachment,
          presigned_url: 'https://storage.yandexcloud.net/test/photo.png',
          expires_at: '2099-01-01T00:00:00Z',
          required_headers: { 'Content-Type': 'image/png', 'x-amz-meta-test': 'signed' },
        },
        201,
      );
    if (path.endsWith('/complete')) return reply(attachment);
    if (path.endsWith('/download-url'))
      return reply({
        url: 'https://storage.yandexcloud.net/test/photo.png',
        expires_at: '2099-01-01T00:00:00Z',
      });
    if (path.endsWith('/confirm')) {
      confirmed = true;
      return reply({ confirmation_count: 2, confirmed_by_me: true });
    }
    if (path.endsWith('/statement')) {
      if (method === 'POST') hasStatement = true;
      return hasStatement
        ? reply(statement, method === 'POST' ? 201 : 200)
        : reply({ error: { code: 'NOT_FOUND' } }, 404);
    }
    if (path.endsWith('/status')) {
      currentIssue = { ...currentIssue, status: req.postDataJSON().new_status };
      return reply(currentIssue);
    }
    if (path.endsWith('/issues/' + ids.issue)) {
      if (options.expired) return reply({ error: { code: 'UNAUTHENTICATED' } }, 401);
      return reply({
        issue: {
          ...currentIssue,
          confirmations_count: confirmed ? 2 : 1,
          created_by: created ? ids.user : ids.other,
        },
        attachments: created ? [{ ...attachment, status: 'ATTACHED', issue_id: ids.issue }] : [],
        confirmed_by_me: confirmed,
        timeline: [],
        latest_statement: hasStatement ? statement : null,
      });
    }
    if (path.endsWith('/issues') && method === 'POST') {
      created = true;
      keys.push(req.headers()['idempotency-key']);
      expect(req.postDataJSON().attachment_ids).toEqual([ids.attachment]);
      currentIssue = { ...issue, ...req.postDataJSON() };
      if (options.ambiguous && ++attempts === 1)
        return reply({ error: { code: 'DEPENDENCY_UNAVAILABLE' } }, 503);
      return reply(currentIssue, 201);
    }
    if (path.endsWith('/issues'))
      return reply({ items: active === ids.house ? [currentIssue] : [], next_page_token: '' });
    if (path.endsWith('/announcements')) {
      if (method === 'POST') {
        sent = true;
        return reply({ ...announcement, ...req.postDataJSON() }, 201);
      }
      return reply({ items: sent ? [announcement] : [], next_page_token: '' });
    }
    throw new Error('Unexpected REST route: ' + method + ' ' + path);
  });
  return { keys };
}
test('resident creates issue through direct upload, complete and POST; ambiguous retry is stable', async ({
  page,
}) => {
  const { keys } = await setup(page, 'RESIDENT', { ambiguous: true });
  await page.goto('/issues/new');
  await page.getByLabel('Что случилось?').fill('Мусор у второго подъезда');
  await page.getByLabel('Где находится проблема?').fill('Второй подъезд');
  await page
    .getByLabel('Добавить фотографии')
    .setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: png });
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Отправить проблему' }).click();
  await expect(page.getByRole('alert')).toContainText('Сервис временно недоступен');
  await page.getByRole('button', { name: 'Повторить отправку' }).click();
  await expect(page).toHaveURL('/issues/' + ids.issue);
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBe(keys[1]);
  await expect(page.getByRole('button', { name: 'Вы сообщили об этом' })).toBeDisabled();
  await expect(page.getByAltText('photo.png')).toBeVisible();
});
test('another resident confirms once and cannot open chairman UI', async ({ page }) => {
  await setup(page);
  await page.goto('/issues/' + ids.issue);
  await page.getByRole('button', { name: 'Подтвердить проблему' }).click();
  await expect(page.getByRole('button', { name: 'Вы подтвердили проблему' })).toBeDisabled();
  await expect(page.getByRole('heading', { name: '2 подтверждений' })).toBeVisible();
  await page.goto('/chairman');
  await expect(page.getByRole('heading', { name: 'Доступ ограничен' })).toBeVisible();
});
test('chairman generates a statement, copies it, confirms status and publishes announcement', async ({
  page,
}) => {
  await setup(page, 'CHAIRMAN');
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: async (text: string) => {
          (window as unknown as { copied: string }).copied = text;
        },
      },
    });
  });
  await page.goto('/chairman');
  await page.getByRole('link').filter({ hasText: issue.description }).click();
  await page.getByRole('button', { name: 'Сформировать заявление' }).click();
  await expect(page.getByText(statement.body)).toBeVisible();
  await page.getByRole('button', { name: 'Копировать текст' }).click();
  await expect(page.getByRole('status')).toContainText('Текст скопирован');
  expect(await page.evaluate(() => (window as unknown as { copied: string }).copied)).toBe(
    statement.body,
  );
  await page.getByLabel('Новый статус').selectOption('CONFIRMING');
  await page.getByRole('button', { name: 'Изменить статус', exact: true }).click();
  await page.getByRole('button', { name: 'Подтвердить', exact: true }).click();
  await expect(page.locator('.status-badge')).toHaveText('Подтверждается');
  await page.getByRole('link', { name: 'Новости', exact: true }).click();
  await page.getByRole('link', { name: 'Создать объявление' }).click();
  await page.getByLabel('Заголовок').fill(announcement.title);
  await page.getByLabel('Текст объявления').fill(announcement.body);
  await page.getByRole('button', { name: 'Опубликовать' }).click();
  await expect(page.getByRole('heading', { name: announcement.title })).toBeVisible();
});
test('house switch clears the previous house data', async ({ page }) => {
  await setup(page);
  await page.goto('/');
  await expect(page.getByText(issue.description)).toBeVisible();
  await page.getByRole('link', { name: 'Профиль', exact: true }).click();
  await page.getByRole('button').filter({ hasText: 'Второй дом' }).click();
  await page.getByRole('button', { name: 'Подтвердить', exact: true }).click();
  await expect(page.getByText('Проблем пока нет')).toBeVisible();
  await expect(page.getByText(issue.description)).toHaveCount(0);
  await expect(page.getByText('ул. Тестовая, 12')).toBeVisible();
});
test('401 removes protected content and offers MAX reauthentication', async ({ page }) => {
  await setup(page, 'RESIDENT', { expired: true });
  await page.goto('/issues/' + ids.issue);
  await expect(page.getByRole('alert')).toContainText('Сессия завершилась');
  await expect(page.getByRole('button', { name: 'Подтвердить проблему' })).toHaveCount(0);
});
test('missing MAX bridge never performs fake login', async ({ page }) => {
  await page.route('https://st.max.ru/**', (r) => r.abort());
  let requests = 0;
  page.on('request', (r) => {
    if (r.url().includes('/api/')) requests++;
  });
  await page.goto('/');
  await expect(page.getByText('Откройте приложение в MAX')).toBeVisible({ timeout: 10000 });
  expect(requests).toBe(0);
});
test('layouts fit 320, 360, 390 and 430px; user HTML stays text', async ({ page }) => {
  await setup(page);
  await page.goto('/');
  for (const width of [320, 360, 390, 430]) {
    await page.setViewportSize({ width, height: 844 });
    await expect(page.getByRole('heading', { name: 'Мой дом', exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
  }
  await page.screenshot({
    path: 'test-results/home-' + test.info().project.name + '.png',
    fullPage: true,
  });
  await page.getByRole('link', { name: 'Сообщить о проблеме', exact: false }).click();
  await page.getByLabel('Что случилось?').fill('<img src=x onerror=alert(1)>');
  await expect(page.getByRole('button', { name: 'Отправить проблему' })).toBeDisabled();
  await page.screenshot({
    path: 'test-results/create-' + test.info().project.name + '.png',
    fullPage: true,
  });
});
