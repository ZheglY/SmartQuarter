import { useEffect, useRef, type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { ArrowLeft, ChevronRight, LoaderCircle, TriangleAlert, Inbox } from 'lucide-react';
import { ApiError } from '../api/client';
import type { Issue, IssueStatus, IssueCategory, TimelineEvent } from '../api/models';
import { categoryLabels, statusLabels, formatDate } from '../utils/presentation';
export function PageHeader({
  title,
  caption,
  back,
  action,
}: {
  title: string;
  caption?: string;
  back?: string;
  action?: ReactNode;
}) {
  const { pathname } = useLocation();
  const parent = pathname.startsWith('/community/')
    ? '/community'
    : pathname.startsWith('/admin/')
      ? '/admin'
      : '/';
  const backPath = back && back !== pathname ? back : pathname !== '/' ? parent : undefined;
  return (
    <header className="page-header">
      <div>
        {backPath && (
          <Link className="back-link" to={backPath}>
            <ArrowLeft size={18} /> Назад
          </Link>
        )}
        <span className="caption">{caption || 'Умный Квартал'}</span>
        <h1 tabIndex={-1}>{title}</h1>
      </div>
      {action}
    </header>
  );
}
export function LoadingState({ text = 'Загружаем данные…' }: { text?: string }) {
  return (
    <div className="state" role="status">
      <LoaderCircle className="spinner" />
      <p>{text}</p>
    </div>
  );
}
export function ErrorState({ error, retry }: { error: unknown; retry?: () => void }) {
  return (
    <div className="error-state" role="alert">
      <TriangleAlert size={22} />
      <div>
        <strong>{error instanceof Error ? error.message : 'Не удалось загрузить данные.'}</strong>
        {error instanceof ApiError && error.requestId && (
          <small>Код запроса: {error.requestId}</small>
        )}
        {retry && (
          <button className="secondary-btn" onClick={retry}>
            Повторить
          </button>
        )}
      </div>
    </div>
  );
}
export function EmptyState({
  title = 'Пока здесь пусто',
  children,
}: {
  title?: string;
  children?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <div className="empty-icon">
        <Inbox />
      </div>
      <h3>{title}</h3>
      {children && <p>{children}</p>}
    </div>
  );
}
export function StatusBadge({ status }: { status: IssueStatus }) {
  return (
    <span className={'status-badge badge-' + status.toLowerCase()}>{statusLabels[status]}</span>
  );
}
export function CategoryBadge({ category }: { category: IssueCategory }) {
  return <span className="category-badge">{categoryLabels[category]}</span>;
}
export function IssueCard({ issue, chairman = false }: { issue: Issue; chairman?: boolean }) {
  return (
    <Link className="issue-card" to={(chairman ? '/chairman' : '') + '/issues/' + issue.id}>
      <span
        className={
          'status ' +
          (issue.status === 'RESOLVED'
            ? 'status-done'
            : issue.status === 'DETECTED'
              ? 'status-open'
              : 'status-progress')
        }
      />
      <div className="issue-content">
        <span className="caption">{categoryLabels[issue.category]}</span>
        <h3>{issue.description}</h3>
        <p>
          {statusLabels[issue.status]} · {issue.confirmations_count} подтверждений
        </p>
        {issue.location_text && <p>{issue.location_text}</p>}
        <small>{formatDate(issue.created_at)}</small>
      </div>
      <ChevronRight size={20} />
    </Link>
  );
}
const eventLabels: Record<string, string> = {
  'issue.created': 'Проблема зарегистрирована',
  'issue.confirmed': 'Житель подтвердил проблему',
  'issue.status_changed': 'Статус изменён',
  'statement.generated': 'Сформирован черновик заявления',
};
export function IssueTimeline({ events }: { events: TimelineEvent[] }) {
  return (
    <section className="issue-block">
      <h2>История проблемы</h2>
      <ol className="timeline">
        {events.map((event) => (
          <li className="timeline-item" key={event.id}>
            <span className="timeline-dot active" />
            <div>
              <strong>{eventLabels[event.type] || 'Обновление проблемы'}</strong>
              <p>{formatDate(event.created_at)}</p>
            </div>
          </li>
        ))}
      </ol>
      {events.length === 0 && <p>Изменений пока нет.</p>}
    </section>
  );
}
export function ConfirmDialog({
  title,
  children,
  onConfirm,
  onCancel,
}: {
  title: string;
  children: ReactNode;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  return (
    <dialog ref={ref} aria-label={title} onCancel={onCancel}>
      <h2>{title}</h2>
      <p>{children}</p>
      <div className="dialog-actions">
        <button className="secondary-btn" autoFocus onClick={onCancel}>
          Отмена
        </button>
        <button className="primary-btn" onClick={onConfirm}>
          Подтвердить
        </button>
      </div>
    </dialog>
  );
}
