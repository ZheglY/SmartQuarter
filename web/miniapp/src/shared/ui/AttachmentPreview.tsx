import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import { ApiError } from '../api/client';
import { trustedStorageURL } from '../../features/uploads/upload';
import { useUser } from '../../features/session/SessionProvider';
import { LoadingState, ErrorState } from './components';
import type { Attachment } from '../api/models';
export function AttachmentPreview({ attachment }: { attachment: Attachment }) {
  const user = useUser();
  const [retries, setRetries] = useState(0);
  const query = useQuery({
    queryKey: ['attachment', user.active_house_id, attachment.id],
    queryFn: async ({ signal }) => {
      const value = await api.download(attachment.id, signal);
      if (new Date(value.expires_at).getTime() <= Date.now())
        throw new ApiError(502, 'INVALID_RESPONSE');
      return { ...value, url: trustedStorageURL(value.url) };
    },
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
    refetchInterval: (q) =>
      q.state.status === 'success' && q.state.data
        ? Math.min(
            2147483647,
            Math.max(1000, new Date(q.state.data.expires_at).getTime() - Date.now() - 10000),
          )
        : false,
  });
  if (query.isPending) return <LoadingState text="Загружаем фото…" />;
  if (query.error)
    return (
      <ErrorState
        error={query.error}
        retry={() => {
          setRetries(0);
          void query.refetch();
        }}
      />
    );
  return (
    <figure className="attachment-preview">
      {retries > 1 ? (
        <ErrorState
          error={new Error('Фото недоступно.')}
          retry={() => {
            setRetries(0);
            void query.refetch();
          }}
        />
      ) : (
        <img
          key={query.dataUpdatedAt}
          src={query.data.url}
          alt={attachment.original_filename}
          referrerPolicy="no-referrer"
          onError={() => {
            if (retries === 0) {
              setRetries(1);
              void query.refetch();
            } else setRetries(2);
          }}
        />
      )}
    </figure>
  );
}
