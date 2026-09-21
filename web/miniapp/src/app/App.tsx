import { useEffect } from 'react';
import { NavLink, Routes, Route, Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { Home, TriangleAlert, Bell, Users, User } from 'lucide-react';
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
function Gate() {
  const s = useSession();
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
  if (!membership(s.context))
    return (
      <>
        <PageHeader title="Нет доступа к дому" />
        <main className="page-content">
          <EmptyState title="Нужен доступ к дому">
            Обратитесь к председателю или выберите дом с активным участием в профиле.
          </EmptyState>
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
  const location = useLocation(),
    navigate = useNavigate();
  useEffect(() => {
    window.scrollTo(0, 0);
    return bindBack(() => {
      if (location.key !== 'default') navigate(-1);
      else navigate('/');
    }, location.pathname !== '/');
  }, [location.pathname, location.key, navigate]);
  return (
    <>
      <div key={useSession().context?.active_house_id}>
        <Outlet />
      </div>
      <nav className="bottom-nav" aria-label="Основная навигация">
        {[
          [Home, '/', 'Главная'],
          [TriangleAlert, '/issues', 'Проблемы'],
          [Bell, '/news', 'Новости'],
          [Users, '/community', 'Сообщество'],
          [User, '/profile', 'Профиль'],
        ].map(([Icon, path, label]) => {
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
    </>
  );
}
export function App() {
  return (
    <div className="app">
      <Routes>
        <Route element={<Gate />}>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/issues" element={<IssuesPage />} />
            <Route path="/issues/new" element={<CreateIssuePage />} />
            <Route path="/issues/:issueId" element={<IssueDetailsPage />} />
            <Route path="/news" element={<NewsPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/community" element={<CommunityPage />} />
            <Route element={<Manager />}>
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
