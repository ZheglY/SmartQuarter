import { useState, type ReactNode } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { NotificationFocus } from '../shared/ui/NotificationFocus';
import { useHouseQuery, useHouseAction, QueryState } from '../features/session/houseWorkflow';
import { useUser } from '../features/session/SessionProvider';
import {
  communityApi as api,
  type CalendarEvent,
  type CalendarInput,
} from '../shared/api/community';
import { canManage, formatDate } from '../shared/utils/presentation';
import { PageHeader, ErrorState, EmptyState, ConfirmDialog } from '../shared/ui/components';

function Frame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <>
      <PageHeader title={title} back="/community" />
      <main className="page-content workflow">{children}</main>
    </>
  );
}
function Paging({
  token,
  next,
  setToken,
}: {
  token: string;
  next?: string;
  setToken: (s: string) => void;
}) {
  return (
    <div className="workflow-links">
      {token && (
        <button className="secondary-btn" onClick={() => setToken('')}>
          В начало списка
        </button>
      )}
      {next && (
        <button className="secondary-btn" onClick={() => setToken(next)}>
          Следующая страница
        </button>
      )}
    </div>
  );
}
export function PollsPage() {
  const manager = canManage(useUser()),
    [status, setStatus] = useState(''),
    [token, setToken] = useState('');
  const q = useHouseQuery(`polls:${status}:${token}`, (s) => api.polls(status, token, s));
  return (
    <Frame title="Опросы">
      {manager && (
        <Link className="primary-btn" to="/community/polls/new">
          Создать опрос
        </Link>
      )}
      <label>
        Показать
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setToken('');
          }}
        >
          <option value="">Все опросы</option>
          <option value="OPEN">Открытые</option>
          <option value="CLOSED">Завершённые</option>
        </select>
      </label>
      <QueryState query={q} />
      {q.data?.items.map((p) => (
        <Link className="news-card" to={'/community/polls/' + p.id} key={p.id}>
          <h2>{p.question}</h2>
          <p>
            {p.status === 'POLL_STATUS_OPEN' ? 'Открыт до' : 'Завершён'} {formatDate(p.ends_at)}
          </p>
          <span>Голосование и результаты →</span>
        </Link>
      ))}
      {q.data && !q.data.items.length && <EmptyState title="Опросов пока нет" />}
      <Paging token={token} next={q.data?.next_page_token} setToken={setToken} />
    </Frame>
  );
}
export function CreatePollPage() {
  const [question, setQuestion] = useState(''),
    [options, setOptions] = useState(['', '']),
    [end, setEnd] = useState(''),
    [error, setError] = useState('');
  const action = useHouseAction(),
    nav = useNavigate();
  return (
    <Frame title="Новый опрос">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const values = options.map((v) => v.trim());
          if (new Set(values.map((v) => v.toLowerCase())).size !== values.length) {
            setError('Варианты ответа должны отличаться.');
            return;
          }
          if (new Date(end).getTime() <= Date.now()) {
            setError('Выберите время завершения в будущем.');
            return;
          }
          setError('');
          action.run(JSON.stringify({ question, values, end }), async (key) => {
            const p = await api.createPoll(
              { question, options: values, ends_at: new Date(end).toISOString() },
              key,
            );
            nav('/community/polls/' + p.id);
          });
        }}
      >
        <fieldset disabled={action.isPending}>
          <label>
            Вопрос
            <input
              required
              maxLength={500}
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
            />
          </label>
          {options.map((v, i) => (
            <div key={i} className="option-row">
              <label>
                Вариант {i + 1}
                <input
                  required
                  maxLength={200}
                  value={v}
                  onChange={(e) =>
                    setOptions(options.map((old, j) => (i === j ? e.target.value : old)))
                  }
                />
              </label>
              {options.length > 2 && (
                <button
                  type="button"
                  aria-label={'Удалить вариант ' + (i + 1)}
                  className="secondary-btn"
                  onClick={() => setOptions(options.filter((_, j) => j !== i))}
                >
                  Удалить
                </button>
              )}
            </div>
          ))}
          {options.length < 10 && (
            <button
              type="button"
              className="secondary-btn"
              onClick={() => setOptions([...options, ''])}
            >
              Добавить вариант
            </button>
          )}
          <label>
            Завершить голосование
            <input
              type="datetime-local"
              required
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </label>
          <p>Дата и время указаны в вашем часовом поясе.</p>
          <button className="primary-btn">Опубликовать опрос</button>
        </fieldset>
      </form>
      {error && <ErrorState error={new Error(error)} />}{' '}
      {action.error && <ErrorState error={action.error} />}
    </Frame>
  );
}
export function PollPage() {
  const { id = '' } = useParams(),
    manager = canManage(useUser()),
    action = useHouseAction(),
    [choice, setChoice] = useState(''),
    [closing, setClosing] = useState(false);
  const q = useHouseQuery('poll:' + id, (s) => api.poll(id, s));
  const d = q.data,
    open = d?.poll.status === 'POLL_STATUS_OPEN' && Date.parse(d.poll.ends_at) > Date.now();
  return (
    <Frame title="Голосование">
      <Link to="/community/polls">Все опросы</Link>
      <QueryState query={q} />
      {d && (
        <>
          <h2>{d.poll.question}</h2>
          <p>
            {open ? 'Голосование до' : 'Голосование завершено'} {formatDate(d.poll.ends_at)}
          </p>
          <p>Всего голосов: {d.total_votes}</p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              action.run('vote:' + id + ':' + choice, (key) => api.vote(id, choice, key));
            }}
          >
            <fieldset disabled={action.isPending || !open || !!d.my_option_id}>
              {d.poll.options.map((o) => {
                const votes = d.results.find((r) => r.option_id === o.id)?.votes_count || 0;
                return (
                  <label className="poll-option" key={o.id}>
                    <span>
                      <input
                        type="radio"
                        name="vote"
                        value={o.id}
                        checked={(d.my_option_id || choice) === o.id}
                        onChange={() => setChoice(o.id)}
                      />
                      {o.text}
                      {d.my_option_id === o.id ? ' · Ваш голос' : ''}
                    </span>
                    <meter min={0} max={d.total_votes || 1} value={votes} />
                    <small>
                      {votes} голосов ·{' '}
                      {d.total_votes ? Math.round((votes * 100) / d.total_votes) : 0}%
                    </small>
                  </label>
                );
              })}
              {open && !d.my_option_id && (
                <button className="primary-btn" disabled={!choice}>
                  Проголосовать
                </button>
              )}
            </fieldset>
          </form>
          {d.my_option_id && <p role="status">Ваш голос учтён. Изменить его нельзя.</p>}
          {manager && open && (
            <button
              className="secondary-btn"
              disabled={action.isPending}
              onClick={() => setClosing(true)}
            >
              Завершить опрос
            </button>
          )}
        </>
      )}
      {action.error && <ErrorState error={action.error} />}{' '}
      {closing && (
        <ConfirmDialog
          title="Завершить голосование?"
          onCancel={() => setClosing(false)}
          onConfirm={() => {
            setClosing(false);
            action.run('close:' + id, (key) => api.closePoll(id, key));
          }}
        >
          Новые голоса больше не принимаются. Результаты сохранятся.
        </ConfirmDialog>
      )}
    </Frame>
  );
}
function localDate(s: string) {
  const d = new Date(s);
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}
export function CalendarPage() {
  const [params] = useSearchParams();
  const manager = canManage(useUser()),
    action = useHouseAction(),
    [month, setMonth] = useState(() => {
      const value = params.get('date') || '';
      const parsed = /^\d{4}-\d{2}-\d{2}$/.test(value) ? new Date(value + 'T12:00:00') : new Date();
      const date = Number.isNaN(parsed.getTime()) ? new Date() : parsed;
      return new Date(date.getFullYear(), date.getMonth(), 1);
    }),
    [editing, setEditing] = useState<CalendarEvent | null | undefined>(),
    [deleting, setDeleting] = useState<CalendarEvent>();
  const from = month.toISOString(),
    to = new Date(month.getFullYear(), month.getMonth() + 1, 1).toISOString();
  const q = useHouseQuery('calendar:' + from, (s) => api.calendar(from, to, s));
  return (
    <Frame title="Календарь дома">
      <NotificationFocus
        ids={q.data?.items.map((i) => i.id) || []}
        loaded={q.isSuccess && !q.isFetching}
      />
      <div className="calendar-toolbar">
        <button
          className="secondary-btn"
          aria-label="Предыдущий месяц"
          onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}
        >
          ←
        </button>
        <h2>{month.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' })}</h2>
        <button
          className="secondary-btn"
          aria-label="Следующий месяц"
          onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}
        >
          →
        </button>
      </div>
      <button
        className="secondary-btn"
        onClick={() => setMonth(new Date(new Date().getFullYear(), new Date().getMonth(), 1))}
      >
        Текущий месяц
      </button>
      {manager && editing === undefined && (
        <button className="primary-btn" onClick={() => setEditing(null)}>
          Добавить событие
        </button>
      )}
      {editing !== undefined && (
        <EventForm
          key={editing?.id || 'new'}
          value={editing}
          pending={action.isPending}
          cancel={() => setEditing(undefined)}
          save={(body) =>
            action.run(JSON.stringify({ id: editing?.id, body }), async (key) => {
              await api.saveEvent(editing?.id || '', body, key);
              setEditing(undefined);
            })
          }
        />
      )}
      <QueryState query={q} />
      {q.data?.items.map((e) => (
        <article className="news-card" key={e.id} id={'entry-' + e.id} tabIndex={-1}>
          <h2>{e.title}</h2>
          <p>
            {formatDate(e.starts_at)} — {formatDate(e.ends_at)}
          </p>
          <p className="preserve-lines">{e.description}</p>
          {manager && (
            <div className="workflow-links">
              <button
                className="secondary-btn"
                disabled={action.isPending}
                onClick={() => setEditing(e)}
              >
                Редактировать
              </button>
              <button
                className="secondary-btn"
                disabled={action.isPending}
                onClick={() => setDeleting(e)}
              >
                Удалить событие
              </button>
            </div>
          )}
        </article>
      ))}
      {q.data && !q.data.items.length && <EmptyState title="В этом месяце событий нет" />}
      {action.error && <ErrorState error={action.error} />}
      {deleting && (
        <ConfirmDialog
          title="Удалить событие?"
          onCancel={() => setDeleting(undefined)}
          onConfirm={() => {
            action.run('delete:' + deleting.id, (key) => api.deleteEvent(deleting.id, key));
            setDeleting(undefined);
          }}
        >
          {deleting.title} исчезнет из календаря дома.
        </ConfirmDialog>
      )}
    </Frame>
  );
}
function EventForm({
  value,
  pending,
  cancel,
  save,
}: {
  value: CalendarEvent | null;
  pending: boolean;
  cancel: () => void;
  save: (v: CalendarInput) => void;
}) {
  const [title, setTitle] = useState(value?.title || ''),
    [description, setDescription] = useState(value?.description || ''),
    [start, setStart] = useState(value ? localDate(value.starts_at) : ''),
    [end, setEnd] = useState(value ? localDate(value.ends_at) : ''),
    [error, setError] = useState('');
  return (
    <form
      className="news-card"
      onSubmit={(e) => {
        e.preventDefault();
        if (new Date(end) <= new Date(start)) {
          setError('Окончание должно быть позже начала.');
          return;
        }
        setError('');
        save({
          title,
          description,
          starts_at: new Date(start).toISOString(),
          ends_at: new Date(end).toISOString(),
        });
      }}
    >
      <h2>{value ? 'Изменить событие' : 'Новое событие'}</h2>
      <fieldset disabled={pending}>
        <label>
          Название
          <input
            required
            maxLength={200}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>
        <label>
          Описание
          <textarea
            maxLength={5000}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <label>
          Начало
          <input
            type="datetime-local"
            required
            value={start}
            onChange={(e) => setStart(e.target.value)}
          />
        </label>
        <label>
          Окончание
          <input
            type="datetime-local"
            required
            value={end}
            onChange={(e) => setEnd(e.target.value)}
          />
        </label>
        <p>Время указано в вашем часовом поясе.</p>
        <div className="workflow-links">
          <button className="primary-btn">Сохранить событие</button>
          <button type="button" className="secondary-btn" onClick={cancel}>
            Отмена
          </button>
        </div>
      </fieldset>
      {error && <ErrorState error={new Error(error)} />}
    </form>
  );
}
export function InitiativesPage() {
  const manager = canManage(useUser()),
    action = useHouseAction(),
    [token, setToken] = useState(''),
    [adding, setAdding] = useState(false),
    [title, setTitle] = useState(''),
    [description, setDescription] = useState(''),
    [closing, setClosing] = useState('');
  const q = useHouseQuery('initiatives:' + token, (s) => api.initiatives(token, s));
  return (
    <Frame title="Инициативы жильцов">
      <NotificationFocus
        ids={q.data?.items.map((i) => i.id) || []}
        loaded={q.isSuccess && !q.isFetching}
        next={q.data?.next_page_token ? () => setToken(q.data!.next_page_token) : undefined}
      />
      <p>
        Предлагайте улучшения и поддерживайте идеи соседей. Поддержать каждую инициативу можно один
        раз.
      </p>
      <button className="primary-btn" onClick={() => setAdding(!adding)}>
        {adding ? 'Отменить создание' : 'Предложить инициативу'}
      </button>
      {adding && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            action.run(JSON.stringify({ title, description }), async (key) => {
              await api.createInitiative({ title, description }, key);
              setTitle('');
              setDescription('');
              setAdding(false);
              setToken('');
            });
          }}
        >
          <fieldset disabled={action.isPending}>
            <label>
              Название
              <input
                required
                maxLength={200}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </label>
            <label>
              Описание
              <textarea
                required
                maxLength={5000}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </label>
            <button className="primary-btn">Опубликовать инициативу</button>
          </fieldset>
        </form>
      )}
      <QueryState query={q} />
      {q.data?.items.map((i) => (
        <article className="news-card" key={i.id} id={'entry-' + i.id} tabIndex={-1}>
          <h2>{i.title}</h2>
          <p className="preserve-lines">{i.description}</p>
          <p>Поддержали: {i.supports_count}</p>
          {i.status === 'INITIATIVE_STATUS_CLOSED' ? (
            <p>Инициатива завершена</p>
          ) : (
            <div className="workflow-links">
              <button
                className="secondary-btn"
                disabled={action.isPending || i.supported_by_me}
                onClick={() => action.run('support:' + i.id, (key) => api.support(i.id, key))}
              >
                {i.supported_by_me ? 'Вы поддержали' : 'Поддержать'}
              </button>
              {manager && (
                <button
                  className="secondary-btn"
                  disabled={action.isPending}
                  onClick={() => setClosing(i.id)}
                >
                  Завершить инициативу
                </button>
              )}
            </div>
          )}
        </article>
      ))}
      {q.data && !q.data.items.length && <EmptyState title="Предложите первую инициативу" />}
      <Paging token={token} next={q.data?.next_page_token} setToken={setToken} />
      {action.error && <ErrorState error={action.error} />}{' '}
      {closing && (
        <ConfirmDialog
          title="Завершить инициативу?"
          onCancel={() => setClosing('')}
          onConfirm={() => {
            action.run('close:' + closing, (key) => api.closeInitiative(closing, key));
            setClosing('');
          }}
        >
          Новая поддержка больше не принимается. Инициатива останется в истории.
        </ConfirmDialog>
      )}
    </Frame>
  );
}
