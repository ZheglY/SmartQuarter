import { api } from '../../shared/api';
import { ApiError } from '../../shared/api/client';
export const MAX_FILE_BYTES = 10 * 1024 * 1024;
export const MAX_FILES = 10;
export function validateFile(file: File) {
  if (
    !file.name.trim() ||
    new TextEncoder().encode(file.name).length > 255 ||
    /[/\\]/.test(file.name) ||
    file.name.includes(String.fromCharCode(0))
  )
    return 'Сократите имя файла и уберите специальные символы.';
  if (!['image/jpeg', 'image/png'].includes(file.type)) return 'Выберите фотографию JPEG или PNG.';
  if (file.size === 0 || file.size > MAX_FILE_BYTES)
    return 'Размер фотографии должен быть от 1 байта до 10 МБ.';
  return null;
}
export function trustedStorageURL(raw: string) {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new ApiError(502, 'S3_ORIGIN');
  }
  const allowed = (
    import.meta.env.VITE_S3_ORIGINS || 'https://storage.yandexcloud.net,http://localhost:19000'
  )
    .split(',')
    .map((s: string) => s.trim());
  const local = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
  if (
    url.username ||
    url.password ||
    !allowed.includes(url.origin) ||
    (url.protocol !== 'https:' &&
      !(local && url.protocol === 'http:' && ['localhost', '127.0.0.1'].includes(url.hostname)))
  )
    throw new ApiError(502, 'S3_ORIGIN');
  return url.href;
}
export async function uploadPhoto(
  file: File,
  signal: AbortSignal,
  onStage: (s: 'uploading' | 'validating') => void,
) {
  const error = validateFile(file);
  if (error) throw new Error(error);
  onStage('uploading');
  const session = await api.createUpload(
    { filename: file.name, mime_type: file.type, size_bytes: file.size },
    signal,
  );
  if (new Date(session.expires_at).getTime() <= Date.now())
    throw new ApiError(409, 'UPLOAD_EXPIRED');
  const url = trustedStorageURL(session.presigned_url);
  for (const name of Object.keys(session.required_headers))
    if (['authorization', 'cookie', 'proxy-authorization'].includes(name.toLowerCase()))
      throw new ApiError(502, 'INVALID_RESPONSE');
  const controller = new AbortController();
  const cancel = () => controller.abort();
  signal.addEventListener('abort', cancel, { once: true });
  if (signal.aborted) controller.abort();
  const timer = setTimeout(cancel, 120000);
  let result: Response;
  try {
    result = await fetch(url, {
      method: 'PUT',
      body: file,
      headers: session.required_headers,
      credentials: 'omit',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
      signal: controller.signal,
    });
  } catch {
    if (signal.aborted) throw new DOMException('Загрузка остановлена.', 'AbortError');
    throw new ApiError(0, 'UPLOAD_FAILED');
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', cancel);
  }
  if (!result.ok) throw new ApiError(result.status, 'UPLOAD_FAILED');
  onStage('validating');
  const attachment = await api.completeUpload(session.upload_id, signal);
  if (attachment.status !== 'READY') throw new ApiError(409, 'UPLOAD_FAILED');
  return attachment;
}
