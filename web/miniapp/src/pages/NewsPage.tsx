import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { useUser } from '../features/session/SessionProvider';
import { useDrafts, type FormDraft } from '../features/issues/drafts';
import { api } from '../shared/api';
import { uncertain } from '../shared/api/client';
import { protectClosing } from '../shared/max/bridge';
import { canManage, formatDate, byteLength } from '../shared/utils/presentation';
import { PageHeader, LoadingState, EmptyState, ErrorState } from '../shared/ui/components';
export function NewsPage() {
  const user = useUser();
  const q = useInfiniteQuery({
    queryKey: ['news', user.active_house_id],
    initialPageParam: '',
    queryFn: ({ pageParam, signal }) => api.announcements(pageParam, signal),
    getNextPageParam: (p) => p.next_page_token || undefined,
  });
  return (
    <>
      <PageHeader title="Новости дома" />
      <main className="page-content">
        {canManage(user) && (
          <Link className="primary-btn full" to="/chairman/announcements/new">
            Создать объявление
          </Link>
        )}
        {q.isPending ? (
          <LoadingState />
        ) : q.error && !q.data ? (
          <ErrorState error={q.error} retry={() => void q.refetch()} />
        ) : (
          <>
            {!q.data?.pages[0]?.items.length && (
              <EmptyState title="Пока нет объявлений">
                Здесь будут новости от председателя.
              </EmptyState>
            )}
            {q.data?.pages
              .flatMap((p) => p.items)
              .map((n) => (
                <article className="news-card" key={n.id}>
                  <small>{formatDate(n.published_at)}</small>
                  <h2>{n.title}</h2>
                  <p className="pre-wrap">{n.body}</p>
                </article>
              ))}
            {q.isFetchNextPageError && <ErrorState error={q.error} />}{' '}
            {q.hasNextPage && (
              <button
                className="secondary-btn full"
                disabled={q.isFetchingNextPage}
                onClick={() => void q.fetchNextPage()}
              >
                Показать ещё
              </button>
            )}
          </>
        )}
      </main>
    </>
  );
}
export function CreateAnnouncementPage() {
  const user = useUser(),
    store = useDrafts(),
    key = user.user.id + ':' + user.active_house_id,
    qc = useQueryClient(),
    navigate = useNavigate();
  const [draft, setDraft] = useState<FormDraft>(
    () => store.announcements.get(key) || { title: '', body: '' },
  );
  const [pending, setPending] = useState(false),
    [error, setError] = useState<unknown>();
  const busy = useRef(false);
  useEffect(() => {
    store.announcements.set(key, draft);
    protectClosing(!!draft.title || !!draft.body);
    return () => protectClosing(false);
  }, [draft, key, store]);
  function change(d: FormDraft) {
    store.announcements.set(key, d);
    setDraft(d);
  }
  async function submit() {
    if (busy.current) return;
    busy.current = true;
    setPending(true);
    setError(undefined);
    const attempt = draft.attempt || {
      key: crypto.randomUUID(),
      title: draft.title,
      body: draft.body,
    };
    change({ ...draft, attempt });
    try {
      await api.announce(attempt.title, attempt.body, attempt.key);
      store.announcements.delete(key);
      await qc.invalidateQueries({ queryKey: ['news', user.active_house_id] });
      navigate('/news', { replace: true });
    } catch (e) {
      setError(e);
      if (!uncertain(e)) change({ ...draft, attempt: undefined });
    } finally {
      busy.current = false;
      setPending(false);
    }
  }
  return (
    <>
      <PageHeader title="Новое объявление" back="/news" />
      <main className="page-content">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <fieldset disabled={pending || !!draft.attempt}>
            <label>
              Заголовок
              <input
                value={draft.title}
                onChange={(e) => change({ ...draft, title: e.target.value })}
                required
              />
              <small>{byteLength(draft.title)} / 200 байт</small>
            </label>
            <label>
              Текст объявления
              <textarea
                value={draft.body}
                rows={8}
                onChange={(e) => change({ ...draft, body: e.target.value })}
                required
              />
              <small>{byteLength(draft.body)} / 10000 байт</small>
            </label>
          </fieldset>
          {!!error && <ErrorState error={error} />}{' '}
          {draft.attempt && !pending && (
            <p className="notice">
              Повторная отправка использует тот же ключ. Можно также проверить ленту объявлений.
            </p>
          )}
          <button
            className="primary-btn full"
            disabled={
              pending ||
              !draft.title.trim() ||
              !draft.body.trim() ||
              byteLength(draft.title) > 200 ||
              byteLength(draft.body) > 10000
            }
          >
            {pending ? 'Публикуем…' : draft.attempt ? 'Повторить публикацию' : 'Опубликовать'}
          </button>
        </form>
      </main>
    </>
  );
}
