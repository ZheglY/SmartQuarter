import { useState } from 'react';
import { Link } from 'react-router-dom';
import { houseApi } from '../shared/api/house';
import { useHouseAction, useHouseQuery, QueryState } from '../features/session/houseWorkflow';
import { PageHeader, ErrorState, EmptyState, ConfirmDialog } from '../shared/ui/components';

export function AdminPage() {
  const access = useHouseQuery('access', (s) => houseApi.GetHouseAccessState(s)),
    action = useHouseAction();
  const [search, setSearch] = useState(''),
    [query, setQuery] = useState(''),
    [houseSearch, setHouseSearch] = useState(''),
    [houseQuery, setHouseQuery] = useState(''),
    [selected, setSelected] = useState<{ id: string; name: string }>(),
    [confirmation, setConfirmation] = useState<{
      title: string;
      body: string;
      action: (key: string) => Promise<unknown>;
    }>();
  const allowed = access.data?.platform_admin === true;
  const users = useHouseQuery(
      'admin-users:' + query,
      (s) => houseApi.ListPlatformUsers({ query }, s),
      allowed,
    ),
    houses = useHouseQuery(
      'admin-houses:' + houseQuery,
      (s) => houseApi.ListAdminHouses({ query: houseQuery }, s),
      allowed,
    );
  return (
    <>
      <PageHeader title="Администрирование" back="/profile" />
      <main className="page-content workflow">
        <QueryState query={access} />
        {access.data && !allowed && <EmptyState title="Доступ только администратору платформы" />}
        {allowed && (
          <>
            <p>
              Назначайте председателей и управляйте домами. Администратор платформы не зависит от
              выбранного дома.
            </p>
            <Link className="management-link" to="/admin/house-registrations">
              Рассмотреть регистрации домов →
            </Link>
            <section>
              <h2>Пользователи</h2>
              <p>
                Пользователь появится после первого входа через MAX. Поиск по имени, MAX ID или ID
                пользователя; показано до 100 результатов.
              </p>
              <form
                className="workflow-links"
                onSubmit={(e) => {
                  e.preventDefault();
                  setQuery(search.trim());
                }}
              >
                <label>
                  Найти пользователя
                  <input
                    value={search}
                    maxLength={255}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </label>
                <button className="secondary-btn">Найти</button>
              </form>
              <QueryState query={users} />
              {users.data?.items.map((u) => (
                <article className="news-card" key={u.id}>
                  <h3>{u.display_name}</h3>
                  <p>MAX ID: {u.max_user_id}</p>
                  <p>
                    {u.can_register_house
                      ? 'Председатель · может регистрировать дома'
                      : 'Гражданин'}{' '}
                    · Управляет домами: {u.managed_houses}
                  </p>
                  <div className="workflow-links">
                    <button
                      className="secondary-btn"
                      disabled={action.isPending}
                      onClick={() =>
                        setConfirmation(
                          u.can_register_house
                            ? {
                                title: 'Снять полномочия председателя?',
                                body: `${u.display_name} станет жителем во всех своих домах. Нерассмотренные регистрации и передачи полномочий будут отменены.`,
                                action: (key) =>
                                  houseApi.RevokeChairmanPermission({ user_id: u.id }, key),
                              }
                            : {
                                title: 'Назначить председателем?',
                                body: `${u.display_name} сможет подавать заявки на регистрацию домов. Для существующего дома назначьте его ниже.`,
                                action: (key) =>
                                  houseApi.GrantChairmanPermission({ user_id: u.id }, key),
                              },
                        )
                      }
                    >
                      {u.can_register_house ? 'Снять полномочия' : 'Назначить председателем'}
                    </button>
                    <button
                      className="secondary-btn"
                      disabled={action.isPending}
                      onClick={() => setSelected({ id: u.id, name: u.display_name })}
                    >
                      {selected?.id === u.id ? 'Выбран для назначения' : 'Выбрать для дома'}
                    </button>
                  </div>
                </article>
              ))}
              {users.data && !users.data.items.length && (
                <EmptyState title="Пользователи не найдены" />
              )}
            </section>
            <section>
              <h2>Председатели домов</h2>
              <p>
                {selected
                  ? `Выбран: ${selected.name}. Укажите дом для назначения.`
                  : 'Выберите пользователя выше, чтобы назначить его дому.'}
              </p>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  setHouseQuery(houseSearch.trim());
                }}
              >
                <label>
                  Найти дом
                  <input
                    maxLength={255}
                    value={houseSearch}
                    onChange={(e) => setHouseSearch(e.target.value)}
                  />
                </label>
                <button className="secondary-btn">Найти дом</button>
              </form>
              <QueryState query={houses} />
              {houses.data?.items.map((h) => (
                <article className="news-card" key={h.id}>
                  <h3>{h.name}</h3>
                  <p>
                    {h.city}, {h.address}
                  </p>
                  <p>Председатель: {h.chairman_display_name || 'не назначен'}</p>
                  <div className="workflow-links">
                    <button
                      className="primary-btn"
                      disabled={!selected || action.isPending || selected.id === h.chairman_user_id}
                      onClick={() =>
                        selected &&
                        setConfirmation({
                          title: 'Назначить председателя дома?',
                          body: `${selected.name} получит управление домом «${h.name}». Текущий председатель останется жителем.`,
                          action: (key) =>
                            houseApi.AssignHouseChairman(
                              { house_id: h.id, target_user_id: selected.id },
                              key,
                            ),
                        })
                      }
                    >
                      Назначить выбранного
                    </button>
                    {h.chairman_user_id && (
                      <button
                        className="secondary-btn"
                        disabled={action.isPending}
                        onClick={() =>
                          setConfirmation({
                            title: 'Снять председателя дома?',
                            body: 'Председатель останется жителем этого дома. Полномочия в других домах сохранятся.',
                            action: (key) => houseApi.RemoveHouseChairman({ house_id: h.id }, key),
                          })
                        }
                      >
                        Снять с этого дома
                      </button>
                    )}
                  </div>
                </article>
              ))}
              {houses.data && !houses.data.items.length && <EmptyState title="Дома не найдены" />}
            </section>
          </>
        )}
        {action.error && <ErrorState error={action.error} />}{' '}
        {action.isSuccess && <p role="status">Изменения сохранены.</p>}
        {confirmation && (
          <ConfirmDialog
            title={confirmation.title}
            onCancel={() => setConfirmation(undefined)}
            onConfirm={() => {
              action.run(confirmation.title + confirmation.body, confirmation.action);
              setConfirmation(undefined);
            }}
          >
            {confirmation.body}
          </ConfirmDialog>
        )}
      </main>
    </>
  );
}
