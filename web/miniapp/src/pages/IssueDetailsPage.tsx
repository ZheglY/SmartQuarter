import { useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useUser } from '../features/session/SessionProvider';
import { api } from '../shared/api';
import { ApiError, uncertain } from '../shared/api/client';
import type { Issue, IssueStatus, StatementDraft } from '../shared/api/models';
import { validID, bridge } from '../shared/max/bridge';
import {
  membership,
  nextStatuses,
  statusLabels,
  formatDate,
  byteLength,
} from '../shared/utils/presentation';
import {
  PageHeader,
  LoadingState,
  ErrorState,
  StatusBadge,
  CategoryBadge,
  IssueTimeline,
  ConfirmDialog,
} from '../shared/ui/components';
import { AttachmentPreview } from '../shared/ui/AttachmentPreview';
export function IssueDetailsPage({ chairman = false }: { chairman?: boolean }) {
  const { issueId = '' } = useParams(),
    user = useUser(),
    qc = useQueryClient();
  const valid = validID(issueId);
  const q = useQuery({
    queryKey: ['issue', user.active_house_id, issueId],
    enabled: valid,
    queryFn: ({ signal }) => api.issue(issueId, signal),
  });
  const confirm = useMutation({
    mutationFn: () => api.confirm(issueId),
    onSuccess: (result) => {
      qc.setQueryData(
        ['issue', user.active_house_id, issueId],
        q.data
          ? {
              ...q.data,
              confirmed_by_me: result.confirmed_by_me,
              issue: { ...q.data.issue, confirmations_count: result.confirmation_count },
            }
          : undefined,
      );
      void qc.invalidateQueries({ queryKey: ['issue', user.active_house_id, issueId] });
      void qc.invalidateQueries({ queryKey: ['issues', user.active_house_id] });
    },
    onError: (e) => {
      if (e instanceof ApiError && [403, 409].includes(e.status)) void q.refetch();
    },
  });
  const issue = q.data?.issue;
  const [title, ...description] = (issue?.description || '').split(/\n\s*\n/);
  return (
    <>
      <PageHeader title="Проблема дома" back={chairman ? '/chairman' : '/issues'} />
      <main className="page-content">
        {!valid ? (
          <ErrorState error={new Error('Некорректный адрес проблемы.')} />
        ) : q.isPending ? (
          <LoadingState />
        ) : q.error ? (
          <ErrorState error={q.error} retry={() => void q.refetch()} />
        ) : issue && q.data ? (
          <>
            <div className="badge-row">
              <CategoryBadge category={issue.category} />
              <StatusBadge status={issue.status} />
            </div>
            <h2 className="detail-title">{title}</h2>
            {description.length > 0 && <p className="preserve-lines">{description.join('\n\n')}</p>}
            <p className="muted">
              {issue.house_address_snapshot}
              {issue.location_text && ' · ' + issue.location_text}
            </p>
            <p className="caption">Создано {formatDate(issue.created_at)}</p>
            <div className="attachments">
              {q.data.attachments.map((a) => (
                <AttachmentPreview key={a.id} attachment={a} />
              ))}
            </div>
            <section className="issue-block">
              <h2>{issue.confirmations_count} подтверждений</h2>
              <p>Подтвердите, если вы тоже столкнулись с этой проблемой.</p>
              <button
                className="primary-btn full"
                disabled={
                  confirm.isPending ||
                  q.data.confirmed_by_me ||
                  issue.created_by === user.user.id ||
                  issue.status === 'RESOLVED'
                }
                onClick={() => confirm.mutate()}
              >
                {confirm.isPending
                  ? 'Подтверждаем…'
                  : q.data.confirmed_by_me
                    ? 'Вы подтвердили проблему'
                    : issue.created_by === user.user.id
                      ? 'Вы сообщили об этом'
                      : issue.status === 'RESOLVED'
                        ? 'Проблема решена'
                        : 'Подтвердить проблему'}
              </button>
              {confirm.error && <ErrorState error={confirm.error} />}
            </section>
            {chairman && <ChairmanActions issue={issue} />}
            <IssueTimeline events={q.data.timeline} />
          </>
        ) : null}
      </main>
    </>
  );
}
function ChairmanActions({ issue }: { issue: Issue }) {
  const user = useUser(),
    qc = useQueryClient(),
    [note, setNote] = useState(''),
    [target, setTarget] = useState<IssueStatus | ''>(''),
    [dialog, setDialog] = useState(false),
    [copy, setCopy] = useState('');
  const attempt = useRef<{ key: string; note: string } | null>(null);
  const statement = useQuery({
    queryKey: ['statement', user.active_house_id, issue.id],
    queryFn: async ({ signal }) => {
      try {
        return await api.statement(issue.id, signal);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  });
  function invalidate() {
    void qc.invalidateQueries({ queryKey: ['issue', user.active_house_id, issue.id] });
    void qc.invalidateQueries({ queryKey: ['issues', user.active_house_id] });
  }
  const generate = useMutation({
    mutationFn: () => {
      attempt.current ??= { key: crypto.randomUUID(), note };
      return api.generate(issue.id, attempt.current.note, attempt.current.key);
    },
    onSuccess: (result: StatementDraft) => {
      attempt.current = null;
      qc.setQueryData(['statement', user.active_house_id, issue.id], result);
      invalidate();
    },
    onError: (e) => {
      if (!uncertain(e)) attempt.current = null;
    },
  });
  const status = useMutation({
    mutationFn: (next: IssueStatus) => api.status(issue.id, next),
    onSuccess: () => {
      setTarget('');
      invalidate();
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) invalidate();
    },
  });
  async function copyText() {
    try {
      await navigator.clipboard.writeText(statement.data!.body);
      setCopy('Текст скопирован. Отправьте обращение самостоятельно.');
    } catch {
      setCopy('Не удалось скопировать автоматически. Выделите текст заявления ниже.');
    }
  }
  function download() {
    const url = URL.createObjectURL(
      new Blob([statement.data!.body], { type: 'text/plain;charset=utf-8' }),
    );
    const a = document.createElement('a');
    a.href = url;
    a.download = 'statement-' + issue.id + '.txt';
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  const allowed = nextStatuses(issue.status, membership(user)!.role);
  return (
    <>
      <section className="issue-block">
        <h2>Заявление</h2>
        <p>Подготовьте черновик, проверьте текст и самостоятельно направьте его адресату.</p>
        {statement.isPending ? (
          <LoadingState />
        ) : statement.error ? (
          <ErrorState error={statement.error} retry={() => void statement.refetch()} />
        ) : statement.data ? (
          <>
            <small>
              Версия {statement.data.version} · {formatDate(statement.data.created_at)}
            </small>
            <pre className="statement-body" tabIndex={0}>
              {statement.data.body}
            </pre>
            <button className="secondary-btn full" onClick={() => void copyText()}>
              Копировать текст
            </button>
            {!bridge()?.platform ? (
              <button className="secondary-btn full" onClick={download}>
                Скачать .txt
              </button>
            ) : (
              <p className="caption">
                В MAX используйте копирование текста. Скачивание файла будет доступно после
                подключения серверной ссылки.
              </p>
            )}
            {copy && <p role="status">{copy}</p>}
          </>
        ) : (
          <p className="muted">Заявление ещё не сформировано.</p>
        )}
        <label>
          Примечание председателя
          <textarea
            rows={3}
            value={note}
            disabled={generate.isPending || !!attempt.current}
            onChange={(e) => setNote(e.target.value)}
          />
          <small>{byteLength(note)} / 4000 байт</small>
        </label>
        <button
          className="primary-btn full"
          disabled={generate.isPending || byteLength(note) > 4000}
          onClick={() => generate.mutate()}
        >
          {generate.isPending
            ? 'Формируем…'
            : attempt.current
              ? 'Повторить запрос'
              : statement.data
                ? 'Создать новую версию'
                : 'Сформировать заявление'}
        </button>
        {generate.error && <ErrorState error={generate.error} />}
      </section>
      <section className="issue-block">
        <h2>Изменить статус</h2>
        {allowed.length ? (
          <>
            <label>
              Новый статус
              <select
                value={target}
                disabled={status.isPending}
                onChange={(e) => setTarget(e.target.value as IssueStatus)}
              >
                <option value="">Выберите статус</option>
                {allowed.map((s) => (
                  <option key={s} value={s}>
                    {statusLabels[s]}
                  </option>
                ))}
              </select>
            </label>
            <button
              className="secondary-btn full"
              disabled={!target || status.isPending || !allowed.includes(target)}
              onClick={() => setDialog(true)}
            >
              Изменить статус
            </button>
          </>
        ) : (
          <p>Нет доступных переходов для вашей роли.</p>
        )}
        {status.error && <ErrorState error={status.error} />}{' '}
        {dialog && target && (
          <ConfirmDialog
            title="Изменить статус?"
            onCancel={() => setDialog(false)}
            onConfirm={() => {
              setDialog(false);
              status.mutate(target);
            }}
          >
            Новый статус: «{statusLabels[target]}». Указывайте отправку обращения только после
            фактической отправки.
          </ConfirmDialog>
        )}
      </section>
    </>
  );
}
