import { useState } from 'react';
import { useUser } from '../features/session/SessionProvider';
import {
  QueryState,
  WorkflowFrame,
  WorkflowStatus,
  useHouseAction,
  useHouseQuery,
} from '../features/session/houseWorkflow';
import { houseApi } from '../shared/api/house';
import { canManage } from '../shared/utils/presentation';
import { EmptyState, ErrorState, ConfirmDialog } from '../shared/ui/components';

export function ReviewRegistrationsPage() {
  const q = useHouseQuery('admin-registrations', (s) => houseApi.ListPendingHouseRegistrations(s)),
    action = useHouseAction(),
    [reason, setReason] = useState(''),
    [approve, setApprove] = useState('');
  return (
    <WorkflowFrame title="Регистрация домов: проверка">
      <p>
        Одобрение создаст дом и назначит заявителя первым председателем. Проверьте адрес и
        полномочия заявителя до принятия решения.
      </p>
      <label>
        Комментарий к решению
        <textarea maxLength={1000} value={reason} onChange={(e) => setReason(e.target.value)} />
      </label>
      <QueryState query={q} />
      {q.data?.items.length === 0 && <EmptyState title="Новых заявок нет" />}
      {q.data?.items.map((r) => (
        <article className="news-card" key={r.id}>
          <h2>{r.requested_name}</h2>
          <p>
            {r.city}, {r.original_address}
          </p>
          <p>Заявитель: {r.applicant_display_name || r.applicant_user_id}</p>
          <div className="workflow-links">
            <button
              className="primary-btn"
              disabled={action.isPending}
              onClick={() => setApprove(r.id)}
            >
              Одобрить
            </button>
            <button
              className="secondary-btn"
              disabled={action.isPending || !reason.trim()}
              onClick={() =>
                action.run('reject-' + r.id + reason, (k) =>
                  houseApi.RejectHouseRegistration({ id: r.id, reason }, k),
                )
              }
            >
              Отклонить с комментарием
            </button>
          </div>
        </article>
      ))}
      {approve && (
        <ConfirmDialog
          title="Зарегистрировать дом?"
          onCancel={() => setApprove('')}
          onConfirm={() => {
            const id = approve;
            setApprove('');
            action.run('approve-' + id, (k) =>
              houseApi.ApproveHouseRegistration({ id, reason }, k),
            );
          }}
        >
          Заявитель получит роль председателя и доступ к управлению этим домом.
        </ConfirmDialog>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function ReviewJoinRequestsPage() {
  const q = useHouseQuery('house-joins', (s) => houseApi.ListHouseJoinRequests(s)),
    action = useHouseAction(),
    [reason, setReason] = useState('');
  return (
    <WorkflowFrame title="Заявки жильцов">
      <label>
        Причина отказа
        <textarea maxLength={1000} value={reason} onChange={(e) => setReason(e.target.value)} />
      </label>
      <QueryState query={q} />
      {q.data?.items.length === 0 && <EmptyState title="Заявок пока нет" />}
      {q.data?.items.map((r) => (
        <article className="news-card" key={r.id}>
          <h2>{r.display_name || 'Житель ' + r.user_id.slice(0, 8)}</h2>
          <WorkflowStatus status={r.status} />
          {r.status.endsWith('_PENDING') && (
            <div className="workflow-links">
              <button
                className="primary-btn"
                disabled={action.isPending}
                onClick={() =>
                  action.run('approve-join-' + r.id, (k) =>
                    houseApi.ApproveJoinRequest({ id: r.id, reason: '' }, k),
                  )
                }
              >
                Одобрить вступление
              </button>
              <button
                className="secondary-btn"
                disabled={action.isPending || !reason.trim()}
                onClick={() =>
                  action.run('reject-join-' + r.id + reason, (k) =>
                    houseApi.RejectJoinRequest({ id: r.id, reason }, k),
                  )
                }
              >
                Отклонить
              </button>
            </div>
          )}
        </article>
      ))}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function InvitationsPage() {
  const q = useHouseQuery('invitations', (s) => houseApi.ListHouseInvitations(s)),
    action = useHouseAction(),
    [hours, setHours] = useState(48),
    [uses, setUses] = useState(1),
    [token, setToken] = useState(''),
    [deepLink, setDeepLink] = useState('');
  return (
    <WorkflowFrame title="Приглашения">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          action.run('invite-' + hours + '-' + uses, async (k) => {
            const r = await houseApi.CreateHouseInvitation(
              { expires_in_hours: hours, max_uses: uses },
              k,
            );
            setToken(r.token);
            setDeepLink(r.deep_link);
          });
        }}
      >
        <fieldset disabled={action.isPending}>
          <label>
            Срок действия, часов
            <input
              type="number"
              min={1}
              max={168}
              required
              value={hours}
              onChange={(e) => setHours(Number(e.target.value))}
            />
          </label>
          <label>
            Количество использований
            <input
              type="number"
              min={1}
              max={100}
              required
              value={uses}
              onChange={(e) => setUses(Number(e.target.value))}
            />
          </label>
          <button className="primary-btn">Создать приглашение</button>
        </fieldset>
      </form>
      {token && (
        <section className="news-card">
          <h2>Код приглашения</h2>
          <p>Сохраните и передайте жителю. Код доступен только сейчас.</p>
          <input
            aria-label="Код приглашения"
            readOnly
            value={token}
            onFocus={(e) => e.target.select()}
          />
          {deepLink && (
            <label>
              Ссылка для MAX
              <input readOnly value={deepLink} onFocus={(e) => e.target.select()} />
            </label>
          )}
          <button
            className="secondary-btn"
            onClick={() => {
              setToken('');
              setDeepLink('');
            }}
          >
            Скрыть код
          </button>
        </section>
      )}
      <QueryState query={q} />
      {q.data?.items.length === 0 && <EmptyState title="Приглашений пока нет" />}
      {q.data?.items.map((r) => (
        <article className="news-card" key={r.id}>
          <WorkflowStatus status={r.status} />
          <p>
            Использовано {r.used_count} из {r.max_uses}
          </p>
          <p>Действует до {r.expires_at && new Date(r.expires_at).toLocaleString('ru')}</p>
          {r.status === 'INVITATION_STATUS_ACTIVE' && (
            <button
              className="secondary-btn"
              disabled={action.isPending}
              onClick={() =>
                action.run('revoke-' + r.id, (k) => houseApi.RevokeHouseInvitation({ id: r.id }, k))
              }
            >
              Отозвать
            </button>
          )}
        </article>
      ))}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function MembersPage() {
  const q = useHouseQuery('members', (s) => houseApi.ListHouseMembers(s)),
    action = useHouseAction(),
    [remove, setRemove] = useState('');
  return (
    <WorkflowFrame title="Жильцы дома">
      <QueryState query={q} />
      {q.data?.items.map((m) => (
        <article className="news-card" key={m.id}>
          <h2>{m.display_name}</h2>
          <p>
            {m.role === 'CHAIRMAN'
              ? 'Председатель'
              : m.role === 'ADMIN'
                ? 'Администратор дома'
                : 'Житель'}
          </p>
          <WorkflowStatus status={m.status} />
          {m.role === 'RESIDENT' && (
            <div className="workflow-links">
              <button
                className="secondary-btn"
                disabled={action.isPending}
                onClick={() =>
                  action.run('membership-' + m.id + '-' + m.status, (k) =>
                    m.status === 'ACTIVE'
                      ? houseApi.DeactivateMembership({ id: m.id }, k)
                      : houseApi.ReactivateMembership({ id: m.id }, k),
                  )
                }
              >
                {m.status === 'ACTIVE' ? 'Приостановить доступ' : 'Восстановить доступ'}
              </button>
              <button
                className="secondary-btn"
                disabled={action.isPending}
                onClick={() => setRemove(m.id)}
              >
                Удалить участие
              </button>
            </div>
          )}
        </article>
      ))}
      {remove && (
        <ConfirmDialog
          title="Удалить участие в доме?"
          onCancel={() => setRemove('')}
          onConfirm={() => {
            const id = remove;
            setRemove('');
            action.run('remove-' + id, (k) => houseApi.RemoveMembership({ id }, k));
          }}
        >
          Житель потеряет доступ к данным дома. Для возвращения потребуется новая заявка.
        </ConfirmDialog>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
export function TransferPage() {
  const user = useUser(),
    manager = canManage(user),
    q = useHouseQuery('transfers', (s) => houseApi.ListChairmanTransfers(s)),
    members = useHouseQuery('members', (s) => houseApi.ListHouseMembers(s), manager),
    action = useHouseAction(),
    [target, setTarget] = useState(''),
    [accept, setAccept] = useState('');
  return (
    <WorkflowFrame title="Роль председателя">
      <p>
        Администратор платформы должен заранее разрешить получателю быть председателем.
        Получатель должен подтвердить предложение в течение 48 часов. После подтверждения текущий
        председатель станет жителем.
      </p>
      {manager && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            action.run('transfer-' + target, (k) =>
              houseApi.CreateChairmanTransfer({ target_user_id: target }, k),
            );
          }}
        >
          <QueryState query={members} />
          <label>
            Новый председатель
            <select required value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Выберите активного жителя</option>
              {members.data?.items
                .filter((m) => m.role === 'RESIDENT' && m.status === 'ACTIVE')
                .map((m) => (
                  <option key={m.id} value={m.user_id}>
                    {m.display_name}
                  </option>
                ))}
            </select>
          </label>
          <button className="primary-btn" disabled={action.isPending || !target}>
            Предложить роль
          </button>
        </form>
      )}
      <QueryState query={q} />
      {q.data?.items.length === 0 && <EmptyState title="Предложений пока нет" />}
      {q.data?.items.map((r) => (
        <article className="news-card" key={r.id}>
          <WorkflowStatus status={r.status} />
          <p>
            {r.target_user_id === user.user.id
              ? 'Вам предложена роль председателя'
              : 'Ваше предложение передачи роли'}
          </p>
          {r.status.endsWith('_PENDING') && (
            <div className="workflow-links">
              {r.target_user_id === user.user.id ? (
                <>
                  <button
                    className="primary-btn"
                    disabled={action.isPending}
                    onClick={() => setAccept(r.id)}
                  >
                    Принять роль
                  </button>
                  <button
                    className="secondary-btn"
                    disabled={action.isPending}
                    onClick={() =>
                      action.run('reject-transfer-' + r.id, (k) =>
                        houseApi.RejectChairmanTransfer({ id: r.id }, k),
                      )
                    }
                  >
                    Отказаться
                  </button>
                </>
              ) : (
                <button
                  className="secondary-btn"
                  disabled={action.isPending}
                  onClick={() =>
                    action.run('cancel-transfer-' + r.id, (k) =>
                      houseApi.CancelChairmanTransfer({ id: r.id }, k),
                    )
                  }
                >
                  Отменить предложение
                </button>
              )}
            </div>
          )}
        </article>
      ))}
      {accept && (
        <ConfirmDialog
          title="Принять роль председателя?"
          onCancel={() => setAccept('')}
          onConfirm={() => {
            const id = accept;
            setAccept('');
            action.run('accept-transfer-' + id, (k) => houseApi.AcceptChairmanTransfer({ id }, k));
          }}
        >
          Вы получите управление домом, заявками жильцов и контактами служб.
        </ConfirmDialog>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
