import { test, expect, type Page } from '@playwright/test';
import { context, ids } from '../fixtures';
const registrationId = 'a1111111-1111-4111-8111-111111111111';
const invitationId = 'a2222222-2222-4222-8222-222222222222';
const contactId = 'a3333333-3333-4333-8333-333333333333';
const transferId = 'a4444444-4444-4444-8444-444444444444';
async function setup(page: Page, role: 'NONE' | 'RESIDENT' | 'CHAIRMAN') {
  await page.route('https://st.max.ru/**', (r) => r.abort());
  await page.addInitScript(() => {
    window.WebApp = {
      initData: 'fixture-signed',
      BackButton: { show() {}, hide() {}, onClick() {}, offClick() {} },
    };
  });
  let currentRole = role;
  let registration = {
    id: registrationId,
    applicant_user_id: ids.user,
    applicant_display_name: 'Житель',
    requested_name: 'Наш дом',
    original_address: 'Лесная, 1',
    city: 'Москва',
    status: 'HOUSE_REGISTRATION_STATUS_PENDING',
    resulting_house_id: '',
    rejection_reason: '',
    created_at: '2026-09-23T00:00:00Z',
  };
  let invitations: Record<string, unknown>[] = [];
  let contacts: Record<string, unknown>[] = [];
  let prefs = {
    notifications_enabled: true,
    issue_notifications_enabled: true,
    announcement_notifications_enabled: true,
    membership_notifications_enabled: true,
    bot_notifications_enabled: true,
  };
  const transfer = {
    id: transferId,
    house_id: ids.house,
    current_chairman_user_id: ids.other,
    target_user_id: ids.user,
    status: 'CHAIRMAN_TRANSFER_STATUS_PENDING',
    created_at: '2026-09-23T00:00:00Z',
    expires_at: '2099-01-01T00:00:00Z',
  };
  await page.route('**/api/v1/**', async (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname.replace('/api/v1', ''),
      method = req.method(),
      reply = (body: unknown, status = 200) => r.fulfill({ json: body, status });
    const uc = {
      ...context,
      active_house_id: currentRole === 'NONE' ? '' : ids.house,
      houses: currentRole === 'NONE' ? [] : context.houses,
      memberships:
        currentRole === 'NONE' ? [] : context.memberships.map((m) => ({ ...m, role: currentRole })),
    };
    if (path === '/session/max')
      return reply({ user_context: uc, expires_at: '2099-01-01T00:00:00Z' });
    if (path === '/me') return reply(uc);
    if (path === '/house-access')
      return reply({
        platform_admin: false,
        can_manage_active_house: currentRole === 'CHAIRMAN',
        pending_registrations: 0,
        pending_join_requests: 0,
        incoming_join_requests: 1,
      });
    if (path === '/house-registrations' && method === 'POST') {
      expect(req.headers()['idempotency-key']).toBeTruthy();
      const body = req.postDataJSON();
      registration = {
        ...registration,
        requested_name: body.name,
        city: body.city,
        original_address: body.address,
      };
      return reply(registration, 202);
    }
    if (path === '/house-registrations/' + registrationId) return reply(registration);
    if (path === '/house-registrations/' + registrationId + '/cancel') {
      registration.status = 'HOUSE_REGISTRATION_STATUS_CANCELLED';
      return reply(registration);
    }
    if (path === '/house-registrations') return reply({ items: [registration] });
    if (path === '/join-requests') return reply({ items: [] });
    if (path === '/chairman/invitations') {
      if (method === 'POST') {
        const v = {
          id: invitationId,
          house_id: ids.house,
          created_by: ids.user,
          expires_at: '2099-01-01T00:00:00Z',
          created_at: '2026-09-23T00:00:00Z',
          max_uses: req.postDataJSON().max_uses,
          used_count: 0,
          status: 'INVITATION_STATUS_ACTIVE',
        };
        invitations = [v];
        return reply(
          {
            invitation: v,
            token: 't'.repeat(43),
            deep_link: 'https://max.ru/test_bot?startapp=invite_' + 't'.repeat(43),
          },
          201,
        );
      }
      return reply({ items: invitations });
    }
    if (path === '/chairman/invitations/' + invitationId + '/revoke') {
      invitations[0].status = 'INVITATION_STATUS_REVOKED';
      return reply(invitations[0]);
    }
    if (path === '/house/service-contacts')
      return reply({
        items: contacts.filter(
          (c) => new URL(req.url()).searchParams.has('include_archived') || c.is_active,
        ),
      });
    if (path === '/chairman/service-contacts' && method === 'POST') {
      const c = { ...req.postDataJSON(), id: contactId, house_id: ids.house, is_active: true };
      contacts = [c];
      return reply(c, 201);
    }
    if (path === '/chairman/service-contacts/' + contactId) {
      contacts[0] =
        method === 'DELETE'
          ? { ...contacts[0], is_active: false }
          : { ...contacts[0], ...req.postDataJSON() };
      return reply(contacts[0]);
    }
    if (path === '/notifications/settings') {
      if (method === 'PUT') prefs = req.postDataJSON();
      return reply(prefs);
    }
    if (path === '/chairman/transfers') return reply({ items: [transfer] });
    if (path === '/chairman/transfers/' + transferId + '/accept') {
      transfer.status = 'CHAIRMAN_TRANSFER_STATUS_ACCEPTED';
      currentRole = 'CHAIRMAN';
      return reply(transfer);
    }
    if (path === '/chairman/members')
      return reply({
        items: [
          {
            id: ids.user,
            user_id: ids.user,
            house_id: ids.house,
            display_name: 'Житель',
            role: currentRole,
            status: 'ACTIVE',
          },
        ],
      });
    return reply({ error: { code: 'NOT_FOUND' } }, 404);
  });
}

test('new user registers and cancels a house without membership', async ({ page }) => {
  await setup(page, 'NONE');
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Мои дома' })).toBeVisible();
  await page.getByRole('link', { name: 'Зарегистрировать дом', exact: true }).click();
  await page.getByLabel('Название дома').fill('Дом на Лесной');
  await page.getByLabel('Город', { exact: true }).fill('Москва');
  await page.getByLabel('Адрес', { exact: true }).fill('Лесная, 1');
  await page.getByRole('button', { name: 'Отправить на рассмотрение' }).click();
  await expect(page.getByText('На рассмотрении', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Отменить заявку' }).click();
  await expect(page.getByText('Отменено', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
});
test('chairman creates a limited invitation and revokes it', async ({ page }) => {
  await setup(page, 'CHAIRMAN');
  await page.goto('/chairman/invitations');
  await page.getByLabel('Количество использований').fill('3');
  await page.getByRole('button', { name: 'Создать приглашение' }).click();
  await expect(page.getByLabel('Код приглашения')).toHaveValue('t'.repeat(43));
  await page.getByRole('button', { name: 'Скрыть код' }).click();
  await expect(page.getByLabel('Код приглашения')).toHaveCount(0);
  await page.getByRole('button', { name: 'Отозвать', exact: true }).click();
  await expect(page.getByText('Отозвано', { exact: true })).toBeVisible();
});
test('chairman creates, edits and archives a service contact', async ({ page }) => {
  await setup(page, 'CHAIRMAN');
  await page.goto('/chairman/service-contacts');
  await page.getByRole('button', { name: 'Добавить контакт' }).click();
  await page.getByLabel('Название', { exact: true }).fill('Диспетчерская');
  await page.getByLabel('Телефон (+7…)', { exact: true }).fill('+79991234567');
  await page.getByRole('button', { name: 'Сохранить контакт' }).click();
  await expect(page.getByRole('link', { name: 'Позвонить +79991234567' })).toHaveAttribute(
    'href',
    'tel:+79991234567',
  );
  await page.getByRole('button', { name: 'Изменить', exact: true }).click();
  await page.getByLabel('Название', { exact: true }).fill('Аварийная служба');
  await page.getByRole('button', { name: 'Сохранить контакт' }).click();
  await expect(page.getByRole('heading', { name: 'Аварийная служба', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'В архив', exact: true }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByRole('heading', { name: 'Аварийная служба · В архиве' })).toBeVisible();
});
test('resident accepts chairman transfer explicitly', async ({ page }) => {
  await setup(page, 'RESIDENT');
  await page.goto('/chairman/transfer');
  await page.getByRole('button', { name: 'Принять роль', exact: true }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Подтвердить' }).click();
  await expect(page.getByText('Принято', { exact: true })).toBeVisible();
});
test('notification preferences persist and remain accessible without membership', async ({
  page,
}) => {
  await setup(page, 'NONE');
  await page.goto('/notifications/settings');
  await page.getByRole('checkbox', { name: 'Объявления и события дома' }).uncheck();
  await page.getByRole('button', { name: 'Сохранить настройки' }).click();
  await expect(page.getByRole('status')).toHaveText('Настройки сохранены.');
  await page.reload();
  await expect(page.getByRole('checkbox', { name: 'Объявления и события дома' })).not.toBeChecked();
});
