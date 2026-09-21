import { describe, it, expect, vi } from 'vitest';
import { api } from '../src/shared/api';
import {
  validateFile,
  uploadPhoto,
  trustedStorageURL,
  MAX_FILE_BYTES,
} from '../src/features/uploads/upload';
import { attachment, ids } from './fixtures';
const file = () => new File(['png'], 'photo.png', { type: 'image/png' });
const session = {
  upload_id: ids.attachment,
  presigned_url: 'https://storage.yandexcloud.net/bucket/photo?signature=test',
  expires_at: '2099-01-01T00:00:00Z',
  required_headers: { 'Content-Type': 'image/png', 'x-amz-meta-test': 'required' },
};
describe('direct object storage upload', () => {
  it('rejects invalid type, empty and oversize files', () => {
    expect(validateFile(new File(['x'], 'a.svg', { type: 'image/svg+xml' }))).toBeTruthy();
    expect(validateFile(new File([], 'empty.png', { type: 'image/png' }))).toBeTruthy();
    expect(
      validateFile(
        new File([new Uint8Array(MAX_FILE_BYTES + 1)], 'big.png', { type: 'image/png' }),
      ),
    ).toBeTruthy();
    expect(validateFile(file())).toBeNull();
  });
  it('rejects non-allowlisted and credential-bearing URLs', () => {
    expect(() => trustedStorageURL('https://evil.example/a')).toThrow();
    expect(() => trustedStorageURL('javascript:alert(1)')).toThrow();
    expect(() => trustedStorageURL('https://user:pass@storage.yandexcloud.net/a')).toThrow();
  });
  it('sends bytes and signed headers without API cookies, then completes', async () => {
    vi.spyOn(api, 'createUpload').mockResolvedValue(session);
    const complete = vi.spyOn(api, 'completeUpload').mockResolvedValue(attachment);
    const fetcher = vi.fn().mockResolvedValue(new Response(null, { status: 200 }));
    vi.stubGlobal('fetch', fetcher);
    const photo = file(),
      stage = vi.fn();
    await expect(uploadPhoto(photo, new AbortController().signal, stage)).resolves.toEqual(
      attachment,
    );
    expect(fetcher).toHaveBeenCalledWith(
      session.presigned_url,
      expect.objectContaining({
        method: 'PUT',
        body: photo,
        headers: session.required_headers,
        credentials: 'omit',
        redirect: 'error',
        referrerPolicy: 'no-referrer',
      }),
    );
    expect(complete).toHaveBeenCalledWith(ids.attachment, expect.any(AbortSignal));
    expect(stage.mock.calls.flat()).toEqual(['uploading', 'validating']);
  });
  it('never completes after failed PUT', async () => {
    vi.spyOn(api, 'createUpload').mockResolvedValue(session);
    const complete = vi.spyOn(api, 'completeUpload');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 403 })));
    await expect(uploadPhoto(file(), new AbortController().signal, vi.fn())).rejects.toThrow();
    expect(complete).not.toHaveBeenCalled();
  });
  it('rejects expired signatures before PUT', async () => {
    vi.spyOn(api, 'createUpload').mockResolvedValue({
      ...session,
      expires_at: '2020-01-01T00:00:00Z',
    });
    const fetcher = vi.fn();
    vi.stubGlobal('fetch', fetcher);
    await expect(uploadPhoto(file(), new AbortController().signal, vi.fn())).rejects.toMatchObject({
      code: 'UPLOAD_EXPIRED',
    });
    expect(fetcher).not.toHaveBeenCalled();
  });
  it('requires READY and rejects credentials in signed headers', async () => {
    vi.spyOn(api, 'createUpload').mockResolvedValue({
      ...session,
      required_headers: { Authorization: 'unexpected' },
    });
    const fetcher = vi.fn();
    vi.stubGlobal('fetch', fetcher);
    await expect(uploadPhoto(file(), new AbortController().signal, vi.fn())).rejects.toThrow();
    expect(fetcher).not.toHaveBeenCalled();
    vi.mocked(api.createUpload).mockResolvedValue(session);
    fetcher.mockResolvedValue(new Response(null, { status: 200 }));
    vi.spyOn(api, 'completeUpload').mockResolvedValue({ ...attachment, status: 'REJECTED' });
    await expect(uploadPhoto(file(), new AbortController().signal, vi.fn())).rejects.toThrow();
  });
});
