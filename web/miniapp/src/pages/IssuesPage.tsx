import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useUser } from '../features/session/SessionProvider';
import { api } from '../shared/api';
import { statuses, type IssueStatus } from '../shared/api/models';
import { statusLabels } from '../shared/utils/presentation';
import {
  PageHeader,
  LoadingState,
  ErrorState,
  EmptyState,
  IssueCard,
} from '../shared/ui/components';
export function IssuesPage({ chairman = false }: { chairman?: boolean }) {
  const user = useUser();
  const [status, setStatus] = useState<IssueStatus | ''>('');
  const query = useInfiniteQuery({
    queryKey: ['issues', user.active_house_id, chairman, status],
    initialPageParam: '',
    queryFn: ({ pageParam, signal }) =>
      api.issues(chairman, pageParam, status ? [status] : [], signal),
    getNextPageParam: (last) => last.next_page_token || undefined,
  });
  return (
    <>
      <PageHeader
        title={chairman ? 'Кабинет председателя' : 'Проблемы дома'}
        caption={chairman ? 'Очередь нерешённых проблем' : undefined}
      />
      <main className="page-content">
        {!chairman && (
          <Link className="primary-btn full" to="/issues/new">
            Сообщить о проблеме
          </Link>
        )}
        <label className="filter-label">
          Статус
          <select value={status} onChange={(e) => setStatus(e.target.value as IssueStatus | '')}>
            <option value="">Все {chairman ? 'нерешённые' : ''}</option>
            {statuses
              .filter((s) => !chairman || s !== 'RESOLVED')
              .map((s) => (
                <option key={s} value={s}>
                  {statusLabels[s]}
                </option>
              ))}
          </select>
        </label>
        {query.isPending ? (
          <LoadingState />
        ) : query.isError && !query.data ? (
          <ErrorState error={query.error} retry={() => void query.refetch()} />
        ) : (
          <>
            {query.data?.pages
              .flatMap((p) => p.items)
              .map((issue) => (
                <IssueCard key={issue.id} issue={issue} chairman={chairman} />
              ))}
            {!query.data?.pages[0]?.items.length && (
              <EmptyState title="Проблем не найдено">Попробуйте другой фильтр.</EmptyState>
            )}
            {query.isFetchNextPageError && <ErrorState error={query.error} />}{' '}
            {query.hasNextPage && (
              <button
                className="secondary-btn full"
                disabled={query.isFetchingNextPage}
                onClick={() => void query.fetchNextPage()}
              >
                {query.isFetchingNextPage ? 'Загружаем…' : 'Показать ещё'}
              </button>
            )}
          </>
        )}
      </main>
    </>
  );
}
