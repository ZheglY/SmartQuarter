import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { House, Plus, ChevronRight } from 'lucide-react';
import { useUser } from '../features/session/SessionProvider';
import { api } from '../shared/api';
import { canManage, membership, roleLabels, formatDate } from '../shared/utils/presentation';
import {
  PageHeader,
  LoadingState,
  ErrorState,
  EmptyState,
  IssueCard,
} from '../shared/ui/components';
export function HomePage() {
  const user = useUser(),
    house = user.houses.find((h) => h.id === user.active_house_id);
  const issues = useQuery({
    queryKey: ['issues', user.active_house_id, 'home'],
    queryFn: ({ signal }) => api.issues(false, '', [], signal, 3),
  });
  const news = useQuery({
    queryKey: ['news', user.active_house_id, 'home'],
    queryFn: ({ signal }) => api.announcements('', signal, 2),
  });
  return (
    <>
      <PageHeader title="Мой дом" caption="Хорошо там, где мы вместе" />
      <main className="page-content">
        <Link to="/profile" className="house-card">
          <House size={26} />
          <div>
            <strong>{house?.name || 'Ваш дом'}</strong>
            <p>{house?.address}</p>
            <span>{roleLabels[membership(user)!.role]}</span>
          </div>
          <ChevronRight />
        </Link>
        <Link className="report-card" to="/issues/new">
          <Plus size={30} />
          <div>
            <h2>Сообщить о проблеме</h2>
            <p>Поможем сделать дом лучше</p>
          </div>
        </Link>
        {canManage(user) && (
          <Link className="management-link" to="/chairman">
            Кабинет председателя <ChevronRight size={18} />
          </Link>
        )}
        <div className="section-heading">
          <h2>Проблемы дома</h2>
          <Link to="/issues">Все</Link>
        </div>
        {issues.isPending ? (
          <LoadingState />
        ) : issues.error ? (
          <ErrorState error={issues.error} retry={() => void issues.refetch()} />
        ) : issues.data.items.length ? (
          issues.data.items.map((i) => <IssueCard key={i.id} issue={i} />)
        ) : (
          <EmptyState title="Проблем пока нет">Здесь появятся обращения жителей.</EmptyState>
        )}
        <div className="section-heading">
          <h2>Объявления</h2>
          <Link to="/news">Все</Link>
        </div>
        {news.isPending ? (
          <LoadingState />
        ) : news.error ? (
          <ErrorState error={news.error} retry={() => void news.refetch()} />
        ) : news.data.items.length ? (
          news.data.items.map((n) => (
            <Link className="news-card" key={n.id} to="/news">
              <small>{formatDate(n.published_at)}</small>
              <h3>{n.title}</h3>
              <p className="clamp">{n.body}</p>
            </Link>
          ))
        ) : (
          <EmptyState title="Пока нет объявлений" />
        )}
      </main>
    </>
  );
}
