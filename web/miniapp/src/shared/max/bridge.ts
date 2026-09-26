// Official MAX Bridge: https://dev.max.ru/docs/webapps/bridge (2026-09-22).
// MAX initializes WebApp itself. No Telegram ready/expand/themeParams calls.
export interface MaxBridge {
  initData: string;
  platform?: string;
  BackButton?: {
    show(): void;
    hide(): void;
    onClick(fn: () => void): void;
    offClick(fn: () => void): void;
  };
  getViewportSize?: () => Promise<{ height: string; width: string }>;
  enableClosingConfirmation?: () => void;
  disableClosingConfirmation?: () => void;
}
declare global {
  interface Window {
    WebApp?: MaxBridge;
  }
}
export function bridge() {
  return window.WebApp;
}
export function rawInitData() {
  return bridge()?.initData || '';
}
export const validID = (v: string) =>
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(v) &&
  v !== '00000000-0000-0000-0000-000000000000';
// start_param is official; issue_<UUID> is our app payload convention, not MAX API.
export function launchIssue(raw: string) {
  const p = new URLSearchParams(raw).get('start_param');
  const id = p?.startsWith('issue_') ? p.slice(6) : '';
  return id && validID(id) ? id : null;
}
export function launchHouseRoute(raw: string) {
  const p = new URLSearchParams(raw).get('start_param') || '';
  const routes: Record<string, string> = {
    houses: '/houses',
    find_house: '/houses/search',
    register_house: '/houses/register',
    my_requests: '/join-requests',
    settings: '/notifications/settings',
    contacts: '/service-contacts',
    join_review: '/chairman/join-requests',
    transfer: '/chairman/transfer',
    poll: '/community/polls',
    initiative: '/community/initiatives',
    calendar: '/community/calendar',
    announcement: '/news',
  };
  if (routes[p]) return routes[p];
  if (/^invite_[A-Za-z0-9_-]{43}$/.test(p))
    return '/invitations/redeem?token=' + encodeURIComponent(p.slice(7));
  return null;
}

export function launchTarget(raw: string): { path: string; houseId?: string } | null {
  const p = new URLSearchParams(raw).get('start_param') || '';
  const [kind, id, houseId, day, ...extra] = p.split('_');
  if (
    !['issue', 'poll', 'initiative', 'announcement', 'calendar'].includes(kind) ||
    !validID(id || '') ||
    (houseId && !validID(houseId)) ||
    extra.length
  )
    return null;
  if (day && (kind !== 'calendar' || !/^\d{10}$/.test(day))) return null;
  let path =
    kind === 'issue'
      ? '/issues/' + id
      : kind === 'poll'
        ? '/community/polls/' + id
        : kind === 'announcement'
          ? '/news?focus=' + id
          : '/community/' + (kind === 'initiative' ? 'initiatives' : 'calendar') + '?focus=' + id;
  if (day) {
    const date = new Date(Number(day) * 1000);
    path +=
      '&date=' +
      date.getFullYear() +
      '-' +
      String(date.getMonth() + 1).padStart(2, '0') +
      '-' +
      String(date.getDate()).padStart(2, '0');
  }
  return { path, houseId };
}
export function bindBack(fn: () => void, visible: boolean) {
  const b = bridge()?.BackButton;
  if (!b) return () => {};
  if (visible) b.show();
  else b.hide();
  b.onClick(fn);
  return () => {
    b.offClick(fn);
    b.hide();
  };
}
export function protectClosing(enabled: boolean) {
  if (enabled) bridge()?.enableClosingConfirmation?.();
  else bridge()?.disableClosingConfirmation?.();
}
export async function initializeViewport() {
  try {
    const size = await bridge()?.getViewportSize?.();
    if (size && /^\d+(\.\d+)?(px)?$/.test(size.height)) {
      document.documentElement.style.setProperty('--max-height', parseFloat(size.height) + 'px');
    }
  } catch {
    /* CSS dvh/env remains the fallback. */
  }
}
