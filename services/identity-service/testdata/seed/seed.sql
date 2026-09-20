-- 1. Тестовый дом (из примеров контракта)
INSERT INTO houses (id, name, address, city, created_at, updated_at)
VALUES (
    '22222222-2222-4222-8222-222222222222',
    'Дом на Ленина',
    'Москва, ул. Ленина, д. 10',
    'Москва',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- 2. Пользователь-житель (Ярослав, max_user_id 123456789)
INSERT INTO users (id, max_user_id, display_name, username, default_house_id, created_at, updated_at)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    123456789,
    'Ярослав',
    'yaroslav',
    '22222222-2222-4222-8222-222222222222',
    NOW(),
    NOW()
) ON CONFLICT (max_user_id) DO NOTHING;

-- 3. Пользователь-председатель (автор объявлений и заявлений из спецификации)
INSERT INTO users (id, max_user_id, display_name, username, default_house_id, created_at, updated_at)
VALUES (
    '77777777-7777-4777-8777-777777777777',
    987654321,
    'Председатель ТСЖ',
    'chairman_lenina',
    '22222222-2222-4222-8222-222222222222',
    NOW(),
    NOW()
) ON CONFLICT (max_user_id) DO NOTHING;

-- 4. Членство для Ярослава (RESIDENT, ACTIVE)
INSERT INTO memberships (id, user_id, house_id, role, status, created_at, updated_at)
VALUES (
    '33333333-3333-4333-8333-333333333333',
    '11111111-1111-4111-8111-111111111111',
    '22222222-2222-4222-8222-222222222222',
    'RESIDENT',
    'ACTIVE',
    NOW(),
    NOW()
) ON CONFLICT (user_id, house_id) DO NOTHING;

-- 5. Членство для Председателя (CHAIRMAN, ACTIVE)
INSERT INTO memberships (id, user_id, house_id, role, status, created_at, updated_at)
VALUES (
    'aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa',
    '77777777-7777-4777-8777-777777777777',
    '22222222-2222-4222-8222-222222222222',
    'CHAIRMAN',
    'ACTIVE',
    NOW(),
    NOW()
) ON CONFLICT (user_id, house_id) DO NOTHING;
