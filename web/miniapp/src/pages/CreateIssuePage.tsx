import { useEffect, useReducer, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useUser } from '../features/session/SessionProvider';
import { useDrafts, type IssueDraft } from '../features/issues/drafts';
import { uploadPhoto, validateFile, MAX_FILES } from '../features/uploads/upload';
import { api } from '../shared/api';
import { uncertain } from '../shared/api/client';
import { categories, type IssueCategory } from '../shared/api/models';
import { categoryLabels, byteLength } from '../shared/utils/presentation';
import { protectClosing } from '../shared/max/bridge';
import { PageHeader, ErrorState } from '../shared/ui/components';
const stages = {
  selected: 'Готово к загрузке',
  uploading: 'Загружаем фото…',
  validating: 'Проверяем файл…',
  ready: 'Фото загружено',
  failed: 'Не удалось загрузить',
};
export function CreateIssuePage() {
  const user = useUser(),
    store = useDrafts(),
    key = user.user.id + ':' + user.active_house_id,
    navigate = useNavigate(),
    qc = useQueryClient();
  const [, render] = useReducer((n) => n + 1, 0);
  const [draft] = useState<IssueDraft>(() => {
    let d = store.issues.get(key);
    if (!d) {
      d = { category: 'OTHER', description: '', location: '', consent: false, photos: [] };
      store.issues.set(key, d);
    }
    return d;
  });
  const [pending, setPending] = useState(false),
    [error, setError] = useState<unknown>();
  const controller = useRef<AbortController | null>(null),
    busy = useRef(false);
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => {
    protectClosing(!!draft.description || !!draft.photos.length);
    return () => protectClosing(false);
  }, [draft.description, draft.photos.length]);
  function change(update: Partial<IssueDraft>) {
    Object.assign(draft, update);
    render();
  }
  const valid =
    !!draft.description.trim() &&
    byteLength(draft.description) <= 8000 &&
    byteLength(draft.location) <= 1000 &&
    draft.photos.length > 0 &&
    draft.photos.length <= MAX_FILES &&
    draft.consent;
  async function submit() {
    if (busy.current || !valid) return;
    busy.current = true;
    setPending(true);
    setError(undefined);
    const c = new AbortController();
    controller.current = c;
    try {
      if (!draft.attempt) {
        for (const photo of draft.photos) {
          if (photo.attachmentId) continue;
          try {
            const result = await uploadPhoto(photo.file, c.signal, (stage) => {
              photo.stage = stage;
              render();
            });
            photo.attachmentId = result.id;
            photo.stage = 'ready';
            render();
          } catch (e) {
            photo.stage = 'failed';
            render();
            throw e;
          }
        }
        draft.attempt = {
          key: crypto.randomUUID(),
          body: {
            category: draft.category,
            description: draft.description,
            location_text: draft.location,
            attachment_ids: draft.photos.map((p) => p.attachmentId!),
          },
        };
        render();
      }
      const issue = await api.createIssue(draft.attempt.body, draft.attempt.key, c.signal);
      for (const p of draft.photos) URL.revokeObjectURL(p.preview);
      store.issues.delete(key);
      await qc.invalidateQueries({ queryKey: ['issues', user.active_house_id] });
      navigate('/issues/' + issue.id, { replace: true });
    } catch (e) {
      setError(e);
      if (!uncertain(e)) draft.attempt = undefined;
    } finally {
      busy.current = false;
      setPending(false);
      render();
    }
  }
  return (
    <>
      <PageHeader title="Сообщить о проблеме" back="/issues" />
      <main className="page-content">
        <p className="intro">
          Опишите, что случилось. Жители смогут подтвердить проблему, а председатель — подготовить
          обращение.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <fieldset disabled={pending || !!draft.attempt}>
            <label>
              Категория
              <select
                value={draft.category}
                onChange={(e) => change({ category: e.target.value as IssueCategory })}
              >
                {categories.map((c) => (
                  <option key={c} value={c}>
                    {categoryLabels[c]}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Что случилось?
              <textarea
                required
                rows={5}
                value={draft.description}
                placeholder="Расскажите о проблеме"
                onChange={(e) => change({ description: e.target.value })}
              />
              <small>{byteLength(draft.description)} / 8000 байт</small>
            </label>
            <label>
              Где находится проблема?
              <input
                value={draft.location}
                placeholder="Например, у подъезда № 2"
                onChange={(e) => change({ location: e.target.value })}
              />
              <small>{byteLength(draft.location)} / 1000 байт</small>
            </label>
            <label className="upload-picker">
              Добавить фотографии
              <input
                type="file"
                accept="image/jpeg,image/png"
                multiple
                onChange={(e) => {
                  const files = Array.from(e.target.files || []);
                  e.target.value = '';
                  if (draft.photos.length + files.length > MAX_FILES) {
                    setError(new Error('Можно добавить не более 10 фотографий.'));
                    return;
                  }
                  const invalid = files.map(validateFile).find(Boolean);
                  if (invalid) {
                    setError(new Error(invalid));
                    return;
                  }
                  setError(undefined);
                  change({
                    photos: [
                      ...draft.photos,
                      ...files.map((file) => ({
                        id: crypto.randomUUID(),
                        file,
                        preview: URL.createObjectURL(file),
                        stage: 'selected' as const,
                      })),
                    ],
                  });
                }}
              />
              <small>JPEG или PNG, до 10 МБ каждый. От 1 до 10 фото.</small>
            </label>
            <div className="photo-grid">
              {draft.photos.map((photo) => (
                <figure key={photo.id}>
                  <img src={photo.preview} alt={photo.file.name} />
                  <figcaption>{stages[photo.stage]}</figcaption>
                  <button
                    type="button"
                    className="text-btn"
                    onClick={() => {
                      URL.revokeObjectURL(photo.preview);
                      change({ photos: draft.photos.filter((p) => p.id !== photo.id) });
                    }}
                  >
                    Удалить фото
                  </button>
                </figure>
              ))}
            </div>
            <label className="check-label">
              <input
                type="checkbox"
                checked={draft.consent}
                onChange={(e) => change({ consent: e.target.checked })}
              />
              Я проверил(а), что фотографии не раскрывают чужие персональные данные.
            </label>
          </fieldset>
          {!!error && <ErrorState error={error} />}{' '}
          {draft.attempt && !pending && (
            <p className="notice">
              Результат отправки ещё не подтверждён. Повторите отправку с теми же данными или{' '}
              <Link to="/issues">проверьте список проблем</Link>.
            </p>
          )}
          <button className="primary-btn full" disabled={!valid || pending} type="submit">
            {pending ? 'Отправляем…' : draft.attempt ? 'Повторить отправку' : 'Отправить проблему'}
          </button>
          {pending && (
            <button
              type="button"
              className="secondary-btn full"
              onClick={() => controller.current?.abort()}
            >
              Остановить отправку
            </button>
          )}
        </form>
      </main>
    </>
  );
}
