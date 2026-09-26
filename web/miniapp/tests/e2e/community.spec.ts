import { test, expect, type Page } from '@playwright/test';
import { context, ids } from '../fixtures';
const pollID = 'b1111111-1111-4111-8111-111111111111',
  optID = 'b2222222-2222-4222-8222-222222222222',
  eventID = 'b3333333-3333-4333-8333-333333333333',
  initiativeID = 'b4444444-4444-4444-8444-444444444444';
async function setup(page: Page, role: 'RESIDENT' | 'CHAIRMAN' | 'ADMIN' | 'NONE') {
  await page.route('https://st.max.ru/**', (r) => r.abort());
  await page.addInitScript(() => {
    window.WebApp = {
      initData: 'test-only',
      BackButton: { show() {}, hide() {}, onClick() {}, offClick() {} },
    };
  });
  const uc = {
    ...context,
    houses: role === 'NONE' || role === 'ADMIN' ? [] : context.houses,
    active_house_id: role === 'NONE' || role === 'ADMIN' ? '' : ids.house,
    memberships:
      role === 'NONE' || role === 'ADMIN' ? [] : context.memberships.map((m) => ({ ...m, role })),
  };
  let vote = '',
    closed = false,
    grant = false,
    chairman = '',
    events: Record<string, unknown>[] = [];
  const initiatives: Record<string, unknown>[] = [];
  let poll = {
    id: pollID,
    question: 'Когда провести собрание?',
    status: 'POLL_STATUS_OPEN',
    ends_at: '2099-01-01T00:00:00Z',
    options: [
      { id: optID, text: 'Вечером', position: 0 },
      { id: ids.other, text: 'Утром', position: 1 },
    ],
  };
  const details = () => ({
    poll: { ...poll, status: closed ? 'POLL_STATUS_CLOSED' : 'POLL_STATUS_OPEN' },
    results: poll.options.map((o) => ({ option_id: o.id, votes_count: vote === o.id ? 1 : 0 })),
    total_votes: vote ? 1 : 0,
    my_option_id: vote,
  });
  await page.route('**/api/v1/**', async (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname.replace('/api/v1', ''),
      method = req.method(),
      reply = (body: unknown, status = 200) => r.fulfill({ json: body, status });
    if (path === '/session/max')
      return reply({ user_context: uc, expires_at: '2099-01-01T00:00:00Z' });
    if (path === '/me') return reply(uc);
    if (path === '/house-access')
      return reply({
        platform_admin: role === 'ADMIN',
        can_register_house: role === 'ADMIN' || role === 'CHAIRMAN',
        can_manage_active_house: role === 'CHAIRMAN',
        pending_registrations: 0,
        pending_join_requests: 0,
        incoming_join_requests: 0,
      });
    if (method !== 'GET') expect(req.headers()['idempotency-key']).toBeTruthy();
    if (path === '/polls' && method === 'GET')
      return reply({ items: [details().poll], next_page_token: '' });
    if (path === '/polls' && method === 'POST') {
      const b = req.postDataJSON();
      poll = {
        ...poll,
        ...b,
        options: b.options.map((text: string, i: number) => ({
          id: i === 0 ? optID : ids.other,
          text,
          position: i,
        })),
      };
      return reply(poll, 201);
    }
    if (path === '/polls/' + pollID) return reply(details());
    if (path.endsWith('/vote')) {
      vote = req.postDataJSON().option_id;
      return reply({ total_votes: 1 });
    }
    if (path.endsWith('/close') && path.startsWith('/polls')) {
      closed = true;
      return reply(details());
    }
    if (path === '/calendar' && method === 'GET') return reply({ items: events });
    if (path === '/calendar' && method === 'POST') {
      const e = { id: eventID, ...req.postDataJSON() };
      events.push(e);
      return reply(e, 201);
    }
    if (path === '/calendar/' + eventID && method === 'PATCH') {
      events = [{ id: eventID, ...req.postDataJSON() }];
      return reply(events[0]);
    }
    if (path === '/calendar/' + eventID && method === 'DELETE') {
      events = [];
      return reply({ id: eventID });
    }
    if (path === '/initiatives' && method === 'GET')
      return reply({ items: initiatives, next_page_token: '' });
    if (path === '/initiatives' && method === 'POST') {
      const i = {
        id: initiativeID,
        ...req.postDataJSON(),
        status: 'INITIATIVE_STATUS_OPEN',
        supports_count: 0,
        supported_by_me: false,
      };
      initiatives.push(i);
      return reply(i, 201);
    }
    if (path.endsWith('/support')) {
      initiatives[0] = { ...initiatives[0], supports_count: 1, supported_by_me: true };
      return reply({ supports_count: 1, supported_by_me: true });
    }
    if (path === '/admin/users')
      return reply({
        items: [
          {
            id: ids.other,
            display_name: 'Анна',
            max_user_id: '100002',
            can_register_house: grant,
            managed_houses: chairman ? 1 : 0,
          },
        ],
      });
    if (path === '/admin/users/' + ids.other + '/chairman') {
      grant = method === 'POST';
      return reply({
        id: ids.other,
        display_name: 'Анна',
        max_user_id: '100002',
        can_register_house: grant,
        managed_houses: 0,
      });
    }
    const h = () => ({
      id: ids.house,
      name: 'Дом на Лесной',
      city: 'Москва',
      address: 'Лесная, 1',
      chairman_user_id: chairman,
      chairman_display_name: chairman ? 'Анна' : '',
    });
    if (path === '/admin/houses') return reply({ items: [h()] });
    if (path === '/admin/houses/' + ids.house + '/chairman') {
      chairman = method === 'PUT' ? ids.other : '';
      return reply(h());
    }
    return reply({ error: { code: 'NOT_FOUND' } }, 404);
  });
}
test('resident votes once and sees results with persistent navigation', async ({ page }) => {
  await setup(page, 'RESIDENT');
  await page.goto('/community');
  await page.getByRole('link', { name: /Опросы/ }).click();
  await page.getByRole('link', { name: /Когда провести собрание/ }).click();
  await page.getByRole('radio', { name: /Вечером/ }).check();
  await page.getByRole('button', { name: 'Проголосовать' }).click();
  await expect(page.getByText('Ваш голос учтён. Изменить его нельзя.')).toBeVisible();
  await expect(page.getByText('Всего голосов: 1')).toBeVisible();
  await expect(page.getByRole('radio').first()).toBeDisabled();
  await expect(page.getByRole('navigation', { name: 'Основная навигация' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Назад', exact: true })).toBeVisible();
});
test('chairman creates and closes poll', async ({ page }) => {
  await setup(page, 'CHAIRMAN');
  await page.goto('/community/polls/new');
  await page.getByLabel('Вопрос').fill('Нужны ли деревья?');
  await page.getByLabel('Вариант 1').fill('Да');
  await page.getByLabel('Вариант 2').fill('Нет');
  await page.getByLabel('Завершить голосование').fill('2098-10-01T12:00');
  await page.getByRole('button', { name: 'Опубликовать опрос' }).click();
  await expect(page.getByRole('heading', { name: 'Нужны ли деревья?' })).toBeVisible();
  await page.getByRole('button', { name: 'Завершить опрос' }).click();
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText(/Голосование завершено/)).toBeVisible();
});
test('chairman creates edits and deletes calendar event', async ({ page }, info) => {
  await setup(page, 'CHAIRMAN');
  await page.goto('/community/calendar');
  await page.getByRole('button', { name: 'Добавить событие' }).click();
  await page.getByLabel('Название', { exact: true }).fill('Собрание жильцов');
  await page.getByLabel('Описание', { exact: true }).fill('Обсудим благоустройство');
  await page.getByLabel('Начало', { exact: true }).fill('2026-09-26T12:00');
  await page.getByLabel('Окончание', { exact: true }).fill('2026-09-26T14:00');
  await page.getByRole('button', { name: 'Сохранить событие' }).click();
  await expect(page.getByRole('heading', { name: 'Собрание жильцов' })).toBeVisible();
  await page.getByRole('button', { name: 'Редактировать' }).click();
  await page.getByLabel('Название', { exact: true }).fill('Собрание перенесено');
  await page.getByRole('button', { name: 'Сохранить событие' }).click();
  await expect(page.getByRole('heading', { name: 'Собрание перенесено' })).toBeVisible();
  await page.screenshot({ path: info.outputPath('calendar.png'), fullPage: true });
  await page.getByRole('button', { name: 'Удалить событие' }).click();
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('В этом месяце событий нет')).toBeVisible();
});
test('resident proposes and supports initiative', async ({ page }) => {
  await setup(page, 'RESIDENT');
  await page.goto('/community/initiatives');
  await page.getByRole('button', { name: 'Предложить инициативу' }).click();
  await page.getByLabel('Название', { exact: true }).fill('Скамейки');
  await page.getByLabel('Описание', { exact: true }).fill('Поставить скамейки во дворе');
  await page.getByRole('button', { name: 'Опубликовать инициативу' }).click();
  await page.getByRole('button', { name: 'Поддержать', exact: true }).click();
  await expect(page.getByText('Поддержали: 1')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Вы поддержали' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Завершить инициативу' })).toHaveCount(0);
});
test('admin without house grants role assigns and removes chairman', async ({ page }, info) => {
  await setup(page, 'ADMIN');
  await page.goto('/profile');
  await page.getByRole('link', { name: 'Администрирование платформы →' }).click();
  await page.getByRole('button', { name: 'Назначить председателем', exact: true }).click();
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByRole('button', { name: 'Снять полномочия' })).toBeVisible();
  await page.getByRole('button', { name: 'Выбрать для дома' }).click();
  await page.getByRole('button', { name: 'Назначить выбранного' }).click();
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('Председатель: Анна')).toBeVisible();
  await page.screenshot({ path: info.outputPath('admin.png'), fullPage: true });
  await page.getByRole('button', { name: 'Снять с этого дома' }).click();
  await page.getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('Председатель: не назначен')).toBeVisible();
});
test('citizen cannot register house or access admin and never loses navigation', async ({
  page,
}) => {
  await setup(page, 'NONE');
  for (const path of [
    '/community/polls',
    '/issues/new',
    '/admin',
    '/houses/register',
    '/unknown',
  ]) {
    await page.goto(path);
    await expect(page.getByRole('navigation', { name: 'Основная навигация' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Назад', exact: true })).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
  }
  await expect(page.getByRole('link', { name: 'Дома', exact: true })).toBeVisible();
  await page.goto('/houses/register');
  await expect(page.getByText('Нужны полномочия председателя')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Отправить на рассмотрение' })).toHaveCount(0);
});
