import { useEffect } from 'react';
import { NavLink, Routes, Route, Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { Home, TriangleAlert, Newspaper, Users, User } from 'lucide-react';
import { useSession } from '../features/session/SessionProvider';
import { canManage, membership } from '../shared/utils/presentation';
import { bindBack } from '../shared/max/bridge';
import { LoadingState, ErrorState, EmptyState, PageHeader } from '../shared/ui/components';
import { HomePage } from '../pages/HomePage';
import { IssuesPage } from '../pages/IssuesPage';
import { IssueDetailsPage } from '../pages/IssueDetailsPage';
import { CreateIssuePage } from '../pages/CreateIssuePage';
import { NewsPage, CreateAnnouncementPage } from '../pages/NewsPage';
import { ProfilePage, CommunityPage } from '../pages/ProfilePage';
import {
  HousesPage,
  RegisterHousePage,
  RegistrationPage,
  SearchHousesPage,
  JoinHousePage,
  MyRequestsPage,
  RedeemInvitationPage,
} from '../pages/HousePages';
import {
  ReviewRegistrationsPage,
  ReviewJoinRequestsPage,
  InvitationsPage,
  MembersPage,
  TransferPage,
} from '../pages/ChairmanPages';
import { NotificationSettingsPage } from '../pages/NotificationSettingsPage';
import { ServiceContactsPage } from '../pages/ServiceContactsPage';
import { AdminPage } from '../pages/AdminPage';
import {
  PollsPage,
  PollPage,
  CreatePollPage,
  CalendarPage,
  InitiativesPage,
} from '../pages/CommunityPages';
function Gate() {
  const s = useSession();
  const { pathname } = useLocation();
  if (s.loading || s.switching)
    return (
      <>
        <PageHeader title="Ваш дом — рядом" />
        <LoadingState text={s.switching ? 'Переключаем дом…' : 'Входим через MAX…'} />
      </>
    );
  if (!s.context)
    return (
      <>
        <PageHeader title="Умный Квартал" />
        <main className="page-content">
          <div className="welcome-mark">
            <Home size={38} />
          </div>
          <h2>Заботимся о доме вместе</h2>
          <p className="intro">Проблемы дома, объявления и обращения — в одном месте.</p>
          {s.outside ? (
            <EmptyState title="Откройте приложение в MAX">
              Войдите через мини-приложение вашего домового бота.
            </EmptyState>
          ) : s.expired ? (
            <ErrorState
              error={new Error('Сессия завершилась. Войдите снова через MAX.')}
              retry={s.retry}
            />
          ) : (
            <ErrorState error={s.error} retry={s.retry} />
          )}
        </main>
      </>
    );
  if (
    !membership(s.context) &&
    !(
      /^\/houses(?:\/|$)/.test(pathname) ||
      [
        '/',
        '/profile',
        '/join-requests',
        '/invitations/redeem',
        '/notifications/settings',
        '/admin/house-registrations',
        '/admin',
        '/chairman/transfer',
      ].includes(pathname)
    )
  )
    return (
      <>
        <PageHeader title="Нет доступа к дому" />
        <main className="page-content">
          <EmptyState title="Нужен доступ к дому">
            Найдите свой дом и отправьте заявку на вступление председателю.
          </EmptyState>
          <Link className="primary-btn" to="/houses">
            Найти свой дом
          </Link>
          <ProfilePage embedded />
        </main>
      </>
    );
  return <Outlet />;
}
function Manager() {
  const { context } = useSession();
  return context && canManage(context) ? (
    <Outlet />
  ) : (
    <>
      <PageHeader title="Доступ ограничен" />
      <main className="page-content">
        <ErrorState error={new Error('Этот раздел доступен только председателю дома.')} />
        <Link to="/">На главную</Link>
      </main>
    </>
  );
}
function Layout() {
  const session = useSession();
  const hasHouse = session.context && membership(session.context);
  const location = useLocation(),
    navigate = useNavigate();
  useEffect(() => {
    window.scrollTo(0, 0);
    return bindBack(() => {
      if (typeof window.history.state?.idx === 'number' && window.history.state.idx > 0)
        navigate(-1);
      else navigate('/');
    }, location.pathname !== '/');
  }, [location.pathname, location.key, navigate]);
  return (
    <>
      <div key={session.context?.active_house_id}>
        <Outlet />
      </div>
      {session.context && (
        <nav className="bottom-nav" aria-label="Основная навигация">
          {(hasHouse
            ? [
                [Home, '/', 'Главная'],
                [TriangleAlert, '/issues', 'Проблемы'],
                [Newspaper, '/news', 'Новости'],
                [Users, '/community', 'Сообщество'],
                [User, '/profile', 'Профиль'],
              ]
            : [
                [Home, '/houses', 'Дома'],
                [Users, '/join-requests', 'Заявки'],
                [User, '/profile', 'Профиль'],
              ]
          ).map(([Icon, path, label]) => {
            const Component = Icon as typeof Home;
            return (
              <NavLink
                key={String(path)}
                to={String(path)}
                end={path === '/'}
                className={({ isActive }) => (isActive ? 'active' : '')}
              >
                <Component aria-hidden />
                <span>{String(label)}</span>
              </NavLink>
            );
          })}
        </nav>
      )}
    </>
  );
}
export function App() {
  const { context } = useSession();
  return (
    <div className="app">
      <Routes>
        <Route element={<Layout />}>
          <Route element={<Gate />}>
            <Route
              path="/"
              element={context && membership(context) ? <HomePage /> : <HousesPage />}
            />
            <Route path="/houses" element={<HousesPage />} />
            <Route path="/houses/register" element={<RegisterHousePage />} />
            <Route path="/houses/register/:id" element={<RegistrationPage />} />
            <Route path="/houses/search" element={<SearchHousesPage />} />
            <Route path="/houses/:houseId/join" element={<JoinHousePage />} />
            <Route path="/join-requests" element={<MyRequestsPage />} />
            <Route path="/invitations/redeem" element={<RedeemInvitationPage />} />
            <Route path="/notifications/settings" element={<NotificationSettingsPage />} />
            <Route path="/admin/house-registrations" element={<ReviewRegistrationsPage />} />
            <Route path="/chairman/transfer" element={<TransferPage />} />
            <Route path="/service-contacts" element={<ServiceContactsPage />} />
            <Route path="/issues" element={<IssuesPage />} />
            <Route path="/issues/new" element={<CreateIssuePage />} />
            <Route path="/issues/:issueId" element={<IssueDetailsPage />} />
            <Route path="/news" element={<NewsPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/community" element={<CommunityPage />} />
            <Route path="/admin" element={<AdminPage />} />
            <Route path="/community/polls" element={<PollsPage />} />
            <Route path="/community/polls/:id" element={<PollPage />} />
            <Route path="/community/calendar" element={<CalendarPage />} />
            <Route path="/community/initiatives" element={<InitiativesPage />} />
            <Route element={<Manager />}>
              <Route path="/community/polls/new" element={<CreatePollPage />} />
              <Route path="/chairman/join-requests" element={<ReviewJoinRequestsPage />} />
              <Route path="/chairman/invitations" element={<InvitationsPage />} />
              <Route path="/chairman/members" element={<MembersPage />} />
              <Route path="/chairman/service-contacts" element={<ServiceContactsPage manage />} />
              <Route path="/chairman" element={<IssuesPage chairman />} />
              <Route path="/chairman/issues/:issueId" element={<IssueDetailsPage chairman />} />
              <Route path="/chairman/announcements/new" element={<CreateAnnouncementPage />} />
            </Route>
            <Route
              path="*"
              element={
                <>
                  <PageHeader title="Страница не найдена" />
                  <main className="page-content">
                    <Link to="/">На главную</Link>
                  </main>
                </>
              }
            />
          </Route>
        </Route>
      </Routes>
    </div>
  );
}
