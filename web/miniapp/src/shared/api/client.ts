import { z } from 'zod';
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public requestId = '',
  ) {
    super(messageFor(status, code));
    this.name = 'ApiError';
  }
}
export function messageFor(status: number, code = ''): string {
  if (code.startsWith('IDEMPOTENCY_'))
    return 'Результат отправки пока неизвестен. Проверьте список, прежде чем создавать новую заявку.';
  if (code === 'INVALID_RESPONSE') return 'Сервис вернул неожиданный ответ. Попробуйте позже.';
  if (code === 'S3_ORIGIN') return 'Хранилище фотографий не настроено. Обратитесь к организатору.';
  if (code === 'UPLOAD_EXPIRED') return 'Срок загрузки истёк. Повторите загрузку фотографии.';
  if (code === 'UPLOAD_FAILED')
    return 'Не удалось загрузить фотографию. Проверьте соединение и повторите.';
  return (
    (
      {
        0: 'Нет связи с сервисом. Проверьте интернет.',
        400: 'Проверьте заполненные поля.',
        401: 'Сессия завершилась. Войдите снова через MAX.',
        403: 'У вас нет доступа к этому действию или дому.',
        404: 'Запись не найдена.',
        409: 'Данные изменились или действие уже выполнено. Обновите страницу.',
        413: 'Файл или запрос слишком большой.',
        429: 'Слишком много запросов. Попробуйте через минуту.',
        503: 'Сервис временно недоступен. Попробуйте позже.',
        504: 'Сервис не успел ответить. Проверьте результат перед повтором.',
      } as Record<number, string>
    )[status] || 'Не удалось выполнить действие. Попробуйте позже.'
  );
}
let onUnauthorized: () => void = () => {};
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
  return () => {
    onUnauthorized = () => {};
  };
}
const base = (import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '');
export async function request<T>(
  path: string,
  schema: z.ZodType<T>,
  options: {
    method?: string;
    body?: unknown;
    key?: string;
    signal?: AbortSignal;
    bootstrap?: boolean;
  } = {},
): Promise<T> {
  const controller = new AbortController();
  const cancel = () => controller.abort();
  options.signal?.addEventListener('abort', cancel, { once: true });
  if (options.signal?.aborted) controller.abort();
  const timer = setTimeout(() => controller.abort(), 20000);
  try {
    const response = await fetch(base + '/api/v1' + path, {
      method: options.method || 'GET',
      credentials: 'include',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        'X-Request-Id': crypto.randomUUID(),
        ...(options.key ? { 'Idempotency-Key': options.key } : {}),
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: controller.signal,
    });
    if (!response.ok) {
      let code = 'HTTP_ERROR',
        rid = response.headers.get('X-Request-Id') || '';
      try {
        const body: unknown = await response.json();
        const e = z
          .object({ error: z.object({ code: z.string(), request_id: z.string().optional() }) })
          .safeParse(body);
        if (e.success) {
          code = e.data.error.code;
          rid = e.data.error.request_id || rid;
        }
      } catch {
        /* no raw server details */
      }
      if (response.status === 401 && !options.bootstrap) onUnauthorized();
      throw new ApiError(response.status, code, rid);
    }
    if (response.status === 204) return schema.parse(undefined);
    let value: unknown;
    try {
      value = await response.json();
    } catch {
      throw new ApiError(502, 'INVALID_RESPONSE');
    }
    const parsed = schema.safeParse(value);
    if (!parsed.success) throw new ApiError(502, 'INVALID_RESPONSE');
    return parsed.data;
  } catch (e) {
    if (e instanceof ApiError) throw e;
    if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
    throw new ApiError(0, 'NETWORK_ERROR');
  } finally {
    clearTimeout(timer);
    options.signal?.removeEventListener('abort', cancel);
  }
}
export function uncertain(error: unknown) {
  return (
    (error instanceof ApiError &&
      (error.status === 0 || error.status >= 500 || error.code.startsWith('IDEMPOTENCY_'))) ||
    (error instanceof DOMException && error.name === 'AbortError')
  );
}
