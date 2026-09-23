import { createContext, useContext, useState, useEffect, useRef, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api } from '../../shared/api';
import { setUnauthorizedHandler } from '../../shared/api/client';
import {
  rawInitData,
  launchIssue,
  launchHouseRoute,
  initializeViewport,
} from '../../shared/max/bridge';
import { useDrafts } from '../issues/drafts';
import type { UserContext } from '../../shared/api/models';
type Session = {
  context: UserContext | undefined;
  loading: boolean;
  expired: boolean;
  outside: boolean;
  error: unknown;
  retry: () => void;
  refresh: () => Promise<unknown>;
  switchHouse: (id: string) => Promise<void>;
  logout: () => Promise<void>;
  switching: boolean;
};
const Context = createContext<Session | null>(null);
export function SessionProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient(),
    navigate = useNavigate(),
    drafts = useDrafts();
  const [raw, setRaw] = useState<string | null>(() => rawInitData() || null);
  const [expired, setExpired] = useState(false),
    [switching, setSwitching] = useState(false);
  const bootstrap = useRef(true);
  const launched = useRef(false);
  useEffect(() => {
    if (raw !== null) return;
    let tries = 0;
    const timer = setInterval(() => {
      const value = rawInitData();
      if (value || ++tries >= 40) {
        setRaw(value);
        clearInterval(timer);
      }
    }, 100);
    return () => clearInterval(timer);
  }, [raw]);
  useEffect(() => {
    if (raw) void initializeViewport();
    return setUnauthorizedHandler(() => {
      setExpired(true);
      bootstrap.current = true;
      void qc.cancelQueries();
      qc.clear();
    });
  }, [qc, raw]);
  const session = useQuery({
    queryKey: ['session'],
    enabled: !!raw && !expired,
    queryFn: async ({ signal }) => {
      if (bootstrap.current) {
        const v = await api.bootstrap(raw!, signal);
        bootstrap.current = false;
        return v.user_context;
      }
      return api.me(signal);
    },
    retry: false,
    staleTime: 30000,
    refetchInterval: 60000,
    refetchOnWindowFocus: true,
  });
  useEffect(() => {
    if (session.data && !launched.current) {
      launched.current = true;
      const id = launchIssue(raw || '');
      if (id) navigate('/issues/' + id, { replace: true });
      else {
        const route = launchHouseRoute(raw || '');
        if (route) navigate(route, { replace: true });
      }
    }
  }, [session.data, raw, navigate]);
  async function switchHouse(id: string) {
    if (switching) return;
    setSwitching(true);
    await qc.cancelQueries();
    try {
      await api.switchHouse(id);
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'session' });
      qc.removeQueries({ queryKey: ['session'] });
      const fresh = await api.me();
      qc.setQueryData(['session'], fresh);
      drafts.clear();
      navigate('/', { replace: true });
    } catch (e) {
      setExpired(true);
      bootstrap.current = false;
      qc.clear();
      throw e;
    } finally {
      setSwitching(false);
    }
  }
  async function logout() {
    await api.logout();
    await qc.cancelQueries();
    qc.clear();
    drafts.clear();
    bootstrap.current = true;
    setExpired(true);
    navigate('/', { replace: true });
  }
  return (
    <Context.Provider
      value={{
        context: expired ? undefined : session.data,
        loading: raw === null || (session.isPending && !!raw && !expired),
        expired,
        outside: raw === '',
        error: session.error,
        retry: () => {
          bootstrap.current = true;
          setExpired(false);
          if (raw === null || !raw) setRaw(rawInitData());
          void qc.invalidateQueries({ queryKey: ['session'] });
        },
        refresh: () => session.refetch(),
        switchHouse,
        logout,
        switching,
      }}
    >
      {children}
    </Context.Provider>
  );
}
export function useSession() {
  const value = useContext(Context);
  if (!value) throw new Error('Session provider required');
  return value;
}
export function useUser() {
  const { context } = useSession();
  if (!context) throw new Error('Authenticated route required');
  return context;
}
