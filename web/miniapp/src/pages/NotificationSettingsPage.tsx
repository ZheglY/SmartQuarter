import { useState } from 'react';
import { houseApi, type NotificationPreferences } from '../shared/api/house';
import {
  useHouseQuery,
  useHouseAction,
  WorkflowFrame,
  QueryState,
} from '../features/session/houseWorkflow';
import { ErrorState } from '../shared/ui/components';
const labels: Record<keyof NotificationPreferences, string> = {
  notifications_enabled: 'Все уведомления',
  issue_notifications_enabled: 'Мои проблемы и заявления',
  announcement_notifications_enabled: 'Объявления и события дома',
  membership_notifications_enabled: 'Заявки и доступ к дому',
  bot_notifications_enabled: 'Доставка через MAX-бота',
};
export function NotificationSettingsPage() {
  const q = useHouseQuery('preferences', (s) => houseApi.GetNotificationPreferences(s)),
    [draft, setDraft] = useState<NotificationPreferences>(),
    action = useHouseAction(),
    value = draft || q.data;
  return (
    <WorkflowFrame title="Уведомления">
      <QueryState query={q} />
      {value && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            action.run('preferences-' + JSON.stringify(value), (k) =>
              houseApi.UpdateNotificationPreferences(value, k),
            );
          }}
        >
          <fieldset disabled={action.isPending}>
            {(Object.keys(labels) as (keyof NotificationPreferences)[]).map((k) => (
              <label className="workflow-check" key={k}>
                <input
                  type="checkbox"
                  checked={value[k]}
                  onChange={(e) => setDraft({ ...value, [k]: e.target.checked })}
                />
                {labels[k]}
              </label>
            ))}
            <button className="primary-btn">Сохранить настройки</button>
          </fieldset>
        </form>
      )}
      {action.isSuccess && <p role="status">Настройки сохранены.</p>}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
