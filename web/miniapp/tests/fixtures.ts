export const ids = {
  user: '11111111-1111-4111-8111-111111111111',
  other: '22222222-2222-4222-8222-222222222222',
  house: '33333333-3333-4333-8333-333333333333',
  house2: '44444444-4444-4444-8444-444444444444',
  issue: '55555555-5555-4555-8555-555555555555',
  attachment: '66666666-6666-4666-8666-666666666666',
  statement: '77777777-7777-4777-8777-777777777777',
  announcement: '88888888-8888-4888-8888-888888888888',
};
export const date = '2026-09-22T12:00:00Z';
export const context = {
  user: {
    id: ids.user,
    max_user_id: '100001',
    display_name: 'Тестовый житель',
    username: 'resident',
    created_at: date,
    updated_at: date,
  },
  houses: [
    { id: ids.house, name: 'Наш дом', address: 'ул. Тестовая, 10', city: 'Тестовый город' },
    { id: ids.house2, name: 'Второй дом', address: 'ул. Тестовая, 12', city: 'Тестовый город' },
  ],
  memberships: [
    {
      id: ids.user,
      user_id: ids.user,
      house_id: ids.house,
      role: 'RESIDENT' as const,
      status: 'ACTIVE' as const,
    },
    {
      id: ids.other,
      user_id: ids.user,
      house_id: ids.house2,
      role: 'RESIDENT' as const,
      status: 'ACTIVE' as const,
    },
  ],
  default_house_id: ids.house,
  active_house_id: ids.house,
};
export const issue = {
  id: ids.issue,
  house_id: ids.house,
  created_by: ids.other,
  house_address_snapshot: 'ул. Тестовая, 10',
  category: 'CLEANLINESS' as const,
  description: 'Мусор у второго подъезда',
  location_text: 'Второй подъезд',
  status: 'DETECTED' as const,
  confirmations_count: 1,
  created_at: date,
  updated_at: date,
  resolved_at: null,
};
export const attachment = {
  id: ids.attachment,
  house_id: ids.house,
  issue_id: null,
  uploaded_by: ids.user,
  original_filename: 'photo.png',
  mime_type: 'image/png',
  size_bytes: 68,
  sha256: 'a'.repeat(64),
  etag: 'etag',
  status: 'READY' as const,
  upload_expires_at: '2099-01-01T00:00:00Z',
  created_at: date,
  updated_at: date,
};
export const statement = {
  id: ids.statement,
  issue_id: ids.issue,
  version: 1,
  status: 'DRAFT',
  body: 'Прошу устранить проблему по адресу ул. Тестовая, 10.',
  chairman_note: '',
  source_snapshot: { issue_id: ids.issue },
  created_by: ids.user,
  created_at: date,
  updated_at: date,
};
export const announcement = {
  id: ids.announcement,
  house_id: ids.house,
  author_user_id: ids.user,
  title: 'Уборка двора',
  body: 'В субботу состоится уборка двора.',
  status: 'PUBLISHED' as const,
  published_at: date,
  created_at: date,
};
