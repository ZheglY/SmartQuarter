import { useRef, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSession, useUser } from './SessionProvider';
import { uncertain } from '../../shared/api/client';
import { ErrorState, LoadingState, PageHeader } from '../../shared/ui/components';
export function useHouseQuery<T>(
  name: string,
  fn: (signal: AbortSignal) => Promise<T>,
  enabled = true,
) {
  const user = useUser();
  return useQuery({
    queryKey: ['house-workflow', user.user.id, user.active_house_id, name],
    queryFn: ({ signal }) => fn(signal),
    enabled,
  });
}
export function useHouseAction() {
  const keys = useRef(new Map<string, string>()),
    qc = useQueryClient(),
    session = useSession();
  const mutation = useMutation({
    mutationFn: async ({
      name,
      action,
    }: {
      name: string;
      action: (key: string) => Promise<unknown>;
    }) => {
      const key = keys.current.get(name) || crypto.randomUUID();
      keys.current.set(name, key);
      try {
        const result = await action(key);
        keys.current.delete(name);
        return result;
      } catch (e) {
        if (!uncertain(e)) keys.current.delete(name);
        throw e;
      }
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ['house-workflow'] });
      await session.refresh();
    },
  });
  return {
    ...mutation,
    run: (name: string, action: (key: string) => Promise<unknown>) => {
      if (!mutation.isPending) mutation.mutate({ name, action });
    },
  };
}
export function WorkflowFrame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <>
      <PageHeader title={title} back="/houses" />
      <main className="page-content workflow">{children}</main>
    </>
  );
}
export function QueryState({
  query,
}: {
  query: { isPending: boolean; error: unknown; refetch: () => unknown };
}) {
  if (query.isPending) return <LoadingState />;
  return query.error ? (
    <ErrorState
      error={query.error}
      retry={() => {
        void query.refetch();
      }}
    />
  ) : null;
}
export function WorkflowStatus({ status }: { status: string }) {
  const short = status.split('_').at(-1) || '';
  const labels: Record<string, string> = {
    PENDING: 'На рассмотрении',
    APPROVED: 'Одобрено',
    REJECTED: 'Отклонено',
    CANCELLED: 'Отменено',
    ACTIVE: 'Активно',
    INACTIVE: 'Приостановлено',
    ACCEPTED: 'Принято',
    EXPIRED: 'Срок истёк',
    REVOKED: 'Отозвано',
    EXHAUSTED: 'Лимит исчерпан',
  };
  return <span className="category-badge">{labels[short] || short}</span>;
}
