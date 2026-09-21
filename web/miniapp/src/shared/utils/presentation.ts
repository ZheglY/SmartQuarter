import type { IssueCategory, IssueStatus, UserContext, Role } from '../api/models';
export const categoryLabels: Record<IssueCategory, string> = {
  SAFETY: 'Безопасность',
  CLEANLINESS: 'Чистота',
  UTILITIES: 'Коммунальные услуги',
  INFRASTRUCTURE: 'Инфраструктура',
  OTHER: 'Другое',
};
export const statusLabels: Record<IssueStatus, string> = {
  DETECTED: 'Новая',
  CONFIRMING: 'Подтверждается',
  READY_FOR_APPEAL: 'Готова к обращению',
  HANDED_TO_CHAIRMAN: 'У председателя',
  MARKED_SENT: 'Обращение отправлено',
  WAITING_RESULT: 'Ожидает результата',
  RESOLVED: 'Решена',
};
export const roleLabels: Record<Role, string> = {
  RESIDENT: 'Житель',
  CHAIRMAN: 'Председатель',
  ADMIN: 'Администратор',
};
export function membership(context: UserContext) {
  return context.memberships.find(
    (m) =>
      m.user_id === context.user.id &&
      m.house_id === context.active_house_id &&
      m.status === 'ACTIVE',
  );
}
export function canManage(context: UserContext) {
  const role = membership(context)?.role;
  return role === 'CHAIRMAN' || role === 'ADMIN';
}
export function formatDate(value: string | null) {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? '—'
    : new Intl.DateTimeFormat('ru-RU', {
        day: 'numeric',
        month: 'long',
        hour: '2-digit',
        minute: '2-digit',
      }).format(date);
}
// UI hints only; Issue Service independently enforces every transition.
export function nextStatuses(current: IssueStatus, role: Role): IssueStatus[] {
  if (role === 'RESIDENT') return [];
  const next: Partial<Record<IssueStatus, IssueStatus>> = {
    DETECTED: 'CONFIRMING',
    CONFIRMING: 'READY_FOR_APPEAL',
    READY_FOR_APPEAL: 'HANDED_TO_CHAIRMAN',
    HANDED_TO_CHAIRMAN: 'MARKED_SENT',
    MARKED_SENT: 'WAITING_RESULT',
    WAITING_RESULT: 'RESOLVED',
  };
  const result: IssueStatus[] = next[current] ? [next[current]!] : [];
  if (current === 'DETECTED' || current === 'CONFIRMING') result.push('RESOLVED');
  if (current === 'RESOLVED' && role === 'ADMIN') result.push('CONFIRMING');
  return result;
}
export const byteLength = (s: string) => new TextEncoder().encode(s).length;
