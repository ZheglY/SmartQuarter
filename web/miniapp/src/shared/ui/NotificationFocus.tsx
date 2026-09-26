import { useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { validID } from '../max/bridge';

export function NotificationFocus({
  ids,
  loaded,
  next,
}: {
  ids: string[];
  loaded: boolean;
  next?: () => void;
}) {
  const [params] = useSearchParams();
  const focus = params.get('focus') || '';
  const completed = useRef('');
  useEffect(() => {
    if (!validID(focus) || !loaded || completed.current === focus) return;
    if (ids.includes(focus)) {
      const node = document.getElementById('entry-' + focus);
      node?.scrollIntoView({ block: 'center' });
      node?.focus({ preventScroll: true });
      completed.current = focus;
    } else if (next) next();
  }, [focus, ids, loaded, next]);
  return validID(focus) && loaded && !ids.includes(focus) && !next ? (
    <p role="status">Запись из уведомления больше не доступна. Ниже — актуальные записи дома.</p>
  ) : null;
}
