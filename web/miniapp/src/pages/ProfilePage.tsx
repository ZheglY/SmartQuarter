import { useState } from 'react';
import { houseApi } from '../shared/api/house';
import { useHouseQuery } from '../features/session/houseWorkflow';
import { Link } from 'react-router-dom';
import { useSession, useUser } from '../features/session/SessionProvider';
import { canManage, membership, roleLabels } from '../shared/utils/presentation';
import { PageHeader, ErrorState, ConfirmDialog } from '../shared/ui/components';
export function ProfilePage({ embedded = false }: { embedded?: boolean }) {
  const user = useUser(),
    session = useSession(),
    [house, setHouse] = useState(''),
    [error, setError] = useState<unknown>(),
    [pending, setPending] = useState(false);
  const access = useHouseQuery('access', (s) => houseApi.GetHouseAccessState(s));
  const role = membership(user)?.role;
  async function changeHouse() {
    const selected = house;
    setHouse('');
    setPending(true);
    try {
      await session.switchHouse(selected);
    } catch (e) {
      setError(e);
    } finally {
      setPending(false);
    }
  }
  async function logout() {
    setPending(true);
    try {
      await session.logout();
    } catch (e) {
      setError(e);
    } finally {
      setPending(false);
    }
  }
  const content = (
    <>
      <section className="profile-card">
        <div className="avatar">{user.user.display_name.slice(0, 1) || 'Я'}</div>
        <h2>{user.user.display_name || 'Житель'}</h2>
        {user.user.username && <p>@{user.user.username}</p>}
        <p>MAX ID: {user.user.max_user_id}</p>
        <span className="category-badge">
          {access.data?.platform_admin
            ? 'Администратор платформы'
            : access.data?.can_register_house
              ? 'Председатель'
              : role
                ? roleLabels[role]
                : 'Гражданин'}
        </span>
        <p>
          {role ? 'Участие в активном доме: активно' : 'Нет активного участия в выбранном доме'}
        </p>
      </section>
      <section className="issue-block">
        <Link className="management-link" to="/houses">
          Дома, заявки и управление →
        </Link>
        <Link className="management-link" to="/notifications/settings">
          Настройки уведомлений →
        </Link>
        {role && (
          <Link className="management-link" to="/service-contacts">
            Контакты служб →
          </Link>
        )}
        {access.data?.platform_admin && (
          <Link className="management-link" to="/admin">
            Администрирование платформы →
          </Link>
        )}
        <h2>Мои дома</h2>
        {user.houses
          .filter((h) =>
            user.memberships.some(
              (m) => m.house_id === h.id && m.user_id === user.user.id && m.status === 'ACTIVE',
            ),
          )
          .map((h) => (
            <button
              key={h.id}
              className={'house-choice ' + (h.id === user.active_house_id ? 'selected' : '')}
              disabled={pending || h.id === user.active_house_id}
              onClick={() => setHouse(h.id)}
            >
              <strong>{h.name}</strong>
              <span>{h.address}</span>
              {h.id === user.active_house_id && <small>Активный дом</small>}
            </button>
          ))}
      </section>
      {canManage(user) && (
        <Link className="management-link" to="/chairman">
          Кабинет председателя →
        </Link>
      )}
      {!!error && <ErrorState error={error} />}
      <button className="secondary-btn full" disabled={pending} onClick={() => void logout()}>
        Выйти из сессии
      </button>
      {house && (
        <ConfirmDialog
          title="Переключить дом?"
          onCancel={() => setHouse('')}
          onConfirm={() => void changeHouse()}
        >
          Открытые формы и загруженные фотографии будут сброшены. Данные нового дома загрузятся с
          сервера.
        </ConfirmDialog>
      )}
    </>
  );
  return embedded ? (
    content
  ) : (
    <>
      <PageHeader title="Мой профиль" />
      <main className="page-content">{content}</main>
    </>
  );
}
export function CommunityPage() {
  return (
    <>
      <PageHeader title="Сообщество" />
      <main className="page-content">
        <h2>Вместе — лучше</h2>
        <p className="intro">Следите за жизнью дома и участвуйте в решении общих проблем.</p>
        <Link className="news-card" to="/news">
          <h3>Объявления дома →</h3>
          <p>Новости от вашего председателя</p>
        </Link>
        {[
          ['Опросы', 'polls', 'Голосуйте и смотрите результаты'],
          ['Календарь', 'calendar', 'Собрания, работы и события дома'],
          ['Инициативы', 'initiatives', 'Предлагайте идеи и поддерживайте соседей'],
        ].map(([title, path, description]) => (
          <Link className="news-card" key={title} to={'/community/' + path}>
            <h3>{title}</h3>
            <p>{description} →</p>
          </Link>
        ))}
      </main>
    </>
  );
}
