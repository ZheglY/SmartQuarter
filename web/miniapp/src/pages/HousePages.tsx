import { useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useSession, useUser } from '../features/session/SessionProvider';
import {
  QueryState,
  WorkflowFrame,
  WorkflowStatus,
  useHouseAction,
  useHouseQuery,
} from '../features/session/houseWorkflow';
import { houseApi } from '../shared/api/house';
import { canManage } from '../shared/utils/presentation';
import { EmptyState, ErrorState } from '../shared/ui/components';

export function HousesPage() {
  const user = useUser(),
    session = useSession(),
    action = useHouseAction();
  const access = useHouseQuery('access', (s) => houseApi.GetHouseAccessState(s));
  return (
    <WorkflowFrame title="Мои дома">
      <p className="intro">
        Найдите свой дом или подайте заявку на регистрацию нового. Доступ появится после одобрения.
      </p>
      <div className="workflow-links">
        <Link className="primary-btn" to="/houses/search">
          Найти дом
        </Link>
        <Link className="secondary-btn" to="/houses/register">
          Зарегистрировать дом
        </Link>
        <Link className="secondary-btn" to="/join-requests">
          Мои заявки
        </Link>
        <Link className="secondary-btn" to="/invitations/redeem">
          Ввести приглашение
        </Link>
        <Link className="secondary-btn" to="/notifications/settings">
          Уведомления
        </Link>
        <Link className="secondary-btn" to="/profile">
          Профиль
        </Link>
      </div>
      <QueryState query={access} />
      {access.data?.platform_admin && (
        <Link className="management-link" to="/admin/house-registrations">
          Заявки на регистрацию домов →
        </Link>
      )}
      {user.houses
        .filter((h) => user.memberships.some((m) => m.house_id === h.id && m.status === 'ACTIVE'))
        .map((h) => (
          <article className="news-card" key={h.id}>
            <h2>{h.name}</h2>
            <p>
              {h.city}, {h.address}
            </p>
            <button
              className="primary-btn"
              disabled={action.isPending || h.id === user.active_house_id}
              onClick={() => action.run('switch-' + h.id, () => session.switchHouse(h.id))}
            >
              {h.id === user.active_house_id ? 'Текущий дом' : 'Выбрать дом'}
            </button>
          </article>
        ))}
      {canManage(user) && (
        <nav className="workflow-links" aria-label="Управление домом">
          <Link to="/chairman/join-requests">
            Заявки жильцов ({access.data?.incoming_join_requests ?? 0})
          </Link>
          <Link to="/chairman/invitations">Приглашения</Link>
          <Link to="/chairman/members">Жильцы</Link>
          <Link to="/chairman/transfer">Передать роль председателя</Link>
          <Link to="/chairman/service-contacts">Контакты служб</Link>
        </nav>
      )}
      <Link to="/chairman/transfer">Предложения роли председателя</Link>
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function RegisterHousePage() {
  const [name, setName] = useState(''),
    [city, setCity] = useState(''),
    [address, setAddress] = useState('');
  const action = useHouseAction(),
    navigate = useNavigate();
  return (
    <WorkflowFrame title="Регистрация дома">
      <p>
        Заявку проверит администратор платформы. После одобрения вы станете первым председателем
        дома.
      </p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          action.run(JSON.stringify({ name, city, address }), async (key) => {
            const r = await houseApi.CreateHouseRegistration({ name, city, address }, key);
            navigate('/houses/register/' + r.id);
          });
        }}
      >
        <fieldset disabled={action.isPending}>
          <label>
            Название дома
            <input
              required
              maxLength={255}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            Город
            <input
              required
              maxLength={100}
              value={city}
              onChange={(e) => setCity(e.target.value)}
            />
          </label>
          <label>
            Адрес
            <input
              required
              maxLength={1000}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
            />
          </label>
          <button className="primary-btn full" type="submit">
            Отправить на рассмотрение
          </button>
        </fieldset>
      </form>
      {action.error && (
        <>
          <ErrorState error={action.error} />
          <Link to={'/houses/search?query=' + encodeURIComponent(address)}>
            Проверить, зарегистрирован ли дом
          </Link>
        </>
      )}
    </WorkflowFrame>
  );
}
export function RegistrationPage() {
  const { id = '' } = useParams(),
    action = useHouseAction(),
    session = useSession();
  const q = useHouseQuery('registration-' + id, (s) => houseApi.GetHouseRegistration({ id }, s));
  return (
    <WorkflowFrame title="Заявка на дом">
      <QueryState query={q} />
      {q.data && (
        <article className="news-card">
          <h2>{q.data.requested_name}</h2>
          <p>
            {q.data.city}, {q.data.original_address}
          </p>
          <WorkflowStatus status={q.data.status} />
          {q.data.rejection_reason && <p>{q.data.rejection_reason}</p>}
          {q.data.status.endsWith('_PENDING') && (
            <button
              disabled={action.isPending}
              className="secondary-btn"
              onClick={() =>
                action.run('cancel-' + id, (k) => houseApi.CancelHouseRegistration({ id }, k))
              }
            >
              Отменить заявку
            </button>
          )}
          {q.data.resulting_house_id && (
            <button
              className="primary-btn"
              disabled={action.isPending}
              onClick={() =>
                action.run('enter', () => session.switchHouse(q.data!.resulting_house_id))
              }
            >
              Открыть дом
            </button>
          )}
        </article>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function SearchHousesPage() {
  const [params] = useSearchParams(),
    [query, setQuery] = useState(params.get('query') || ''),
    [search, setSearch] = useState(params.get('query') || '');
  const q = useHouseQuery(
    'search-' + search,
    (s) => houseApi.SearchHouses({ query: search }, s),
    !!search,
  );
  return (
    <WorkflowFrame title="Найти дом">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setSearch(query.trim());
        }}
      >
        <label>
          Город, улица или название
          <input
            required
            maxLength={255}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <button className="primary-btn" type="submit">
          Найти
        </button>
      </form>
      {search && <QueryState query={q} />}
      {q.data?.items.length === 0 && (
        <EmptyState title="Дом не найден">
          Попробуйте другой адрес или зарегистрируйте дом.
        </EmptyState>
      )}
      {q.data?.items.map((h) => (
        <article key={h.id} className="news-card">
          <h2>{h.name}</h2>
          <p>
            {h.city}, {h.address}
          </p>
          <p>{h.has_chairman ? 'Председатель назначен' : 'Председатель пока не назначен'}</p>
          {h.join_available ? (
            <Link className="primary-btn" to={'/houses/' + h.id + '/join'}>
              Подать заявку на вступление
            </Link>
          ) : (
            <span>Вы уже участник дома</span>
          )}
        </article>
      ))}
      <Link to="/houses/register">Зарегистрировать новый дом</Link>
    </WorkflowFrame>
  );
}
export function JoinHousePage() {
  const { houseId = '' } = useParams(),
    action = useHouseAction();
  return (
    <WorkflowFrame title="Вступить в дом">
      <p>Председатель рассмотрит вашу заявку. До одобрения данные дома будут недоступны.</p>
      <button
        className="primary-btn"
        disabled={action.isPending || action.isSuccess}
        onClick={() =>
          action.run('join-' + houseId, (k) => houseApi.CreateJoinRequest({ house_id: houseId }, k))
        }
      >
        Подать заявку
      </button>
      {action.isSuccess && (
        <p role="status">
          Заявка отправлена. <Link to="/join-requests">Посмотреть статус</Link>
        </p>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function MyRequestsPage() {
  const registrations = useHouseQuery('registrations', (s) => houseApi.ListMyHouseRegistrations(s)),
    joins = useHouseQuery('joins', (s) => houseApi.ListMyJoinRequests(s)),
    session = useSession(),
    action = useHouseAction();
  return (
    <WorkflowFrame title="Мои заявки">
      <button
        className="secondary-btn"
        onClick={() => {
          void registrations.refetch();
          void joins.refetch();
        }}
      >
        Обновить статусы
      </button>
      <h2>Регистрация домов</h2>
      <QueryState query={registrations} />
      {registrations.data?.items.length === 0 && <EmptyState title="Заявок на дом пока нет" />}
      {registrations.data?.items.map((r) => (
        <Link className="news-card" key={r.id} to={'/houses/register/' + r.id}>
          <h3>{r.requested_name}</h3>
          <p>
            {r.city}, {r.original_address}
          </p>
          <WorkflowStatus status={r.status} />
        </Link>
      ))}
      <h2>Вступление в дом</h2>
      <QueryState query={joins} />
      {joins.data?.items.length === 0 && <EmptyState title="Заявок на вступление пока нет" />}
      {joins.data?.items.map((r) => (
        <article className="news-card" key={r.id}>
          <p>Заявка № {r.id.slice(0, 8)}</p>
          <WorkflowStatus status={r.status} />
          {r.rejection_reason && <p>{r.rejection_reason}</p>}
          {r.status.endsWith('_PENDING') && (
            <button
              className="secondary-btn"
              disabled={action.isPending}
              onClick={() =>
                action.run('cancel-join-' + r.id, (k) =>
                  houseApi.CancelJoinRequest({ id: r.id }, k),
                )
              }
            >
              Отменить заявку
            </button>
          )}
          {r.status.endsWith('_APPROVED') && (
            <button
              className="primary-btn"
              disabled={action.isPending}
              onClick={() =>
                action.run('enter-' + r.house_id, () => session.switchHouse(r.house_id))
              }
            >
              Выбрать дом
            </button>
          )}
        </article>
      ))}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function RedeemInvitationPage() {
  const [params] = useSearchParams(),
    [token, setToken] = useState(params.get('token') || ''),
    [preview, setPreview] = useState(''),
    action = useHouseAction();
  return (
    <WorkflowFrame title="Приглашение в дом">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          action.run('preview-' + token, async (k) => {
            const h = await houseApi.PreviewHouseInvitation({ token }, k);
            setPreview(`${h.name}, ${h.city}, ${h.address}`);
          });
        }}
      >
        <label>
          Код приглашения
          <input
            required
            autoComplete="off"
            maxLength={64}
            value={token}
            onChange={(e) => {
              setToken(e.target.value.trim());
              setPreview('');
              action.reset();
            }}
          />
        </label>
        <button className="secondary-btn" disabled={action.isPending}>
          Проверить приглашение
        </button>
      </form>
      {preview && (
        <article className="news-card">
          <h2>{preview}</h2>
          <p>Вступление требует одобрения председателя.</p>
          <button
            className="primary-btn"
            disabled={action.isPending}
            onClick={() =>
              action.run('redeem-' + token, async (k) => {
                await houseApi.RedeemHouseInvitation({ token }, k);
                setPreview('');
                setToken('');
              })
            }
          >
            Принять приглашение
          </button>
        </article>
      )}
      {!token && action.isSuccess && (
        <p role="status">
          Заявка отправлена. <Link to="/join-requests">Мои заявки</Link>
        </p>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
