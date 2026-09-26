import { useState } from 'react';
import {
  contactsApi,
  contactCategories,
  contactInputSchema,
  type ContactInput,
} from '../shared/api/contacts';
import {
  WorkflowFrame,
  QueryState,
  useHouseAction,
  useHouseQuery,
} from '../features/session/houseWorkflow';
import { EmptyState, ErrorState, ConfirmDialog } from '../shared/ui/components';
const empty: ContactInput = {
  category: 'OTHER',
  title: '',
  organization_name: '',
  phone: '',
  additional_phone: '',
  email: '',
  website: '',
  description: '',
  emergency: false,
  sort_order: 0,
};
export function ServiceContactsPage({ manage = false }: { manage?: boolean }) {
  const q = useHouseQuery('contacts-' + manage, (s) => contactsApi.list(manage, s)),
    action = useHouseAction(),
    [draft, setDraft] = useState<ContactInput>(),
    [id, setId] = useState(''),
    [archive, setArchive] = useState(''),
    [validation, setValidation] = useState('');
  const fields: [keyof ContactInput, string, number][] = [
    ['title', 'Название', 150],
    ['organization_name', 'Организация', 255],
    ['phone', 'Телефон (+7…)', 16],
    ['additional_phone', 'Дополнительный телефон', 16],
    ['email', 'Электронная почта', 254],
    ['website', 'Сайт (https://…)', 2048],
    ['description', 'Описание', 2000],
  ];
  return (
    <WorkflowFrame
      title={manage ? 'Управление контактами' : 'Контакты служб'}
      back={manage ? '/houses' : '/profile'}
    >
      {manage && !draft && (
        <button
          className="primary-btn"
          onClick={() => {
            setId('');
            setDraft({ ...empty });
          }}
        >
          Добавить контакт
        </button>
      )}
      {draft && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const parsed = contactInputSchema.safeParse(draft);
            if (!parsed.success) {
              setValidation(
                'Проверьте поля. Телефон должен иметь международный формат, например +79991234567.',
              );
              return;
            }
            setValidation('');
            action.run('contact-' + id + JSON.stringify(draft), async (k) => {
              if (id) await contactsApi.update(id, parsed.data, k);
              else await contactsApi.create(parsed.data, k);
              setDraft(undefined);
            });
          }}
        >
          <fieldset disabled={action.isPending}>
            <label>
              Категория
              <select
                value={draft.category}
                onChange={(e) =>
                  setDraft({ ...draft, category: e.target.value as ContactInput['category'] })
                }
              >
                {Object.entries(contactCategories).map(([v, label]) => (
                  <option value={v} key={v}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            {fields.map(([name, label, max]) => (
              <label key={name}>
                {label}
                <input
                  required={name === 'title' || name === 'phone'}
                  maxLength={max}
                  type={
                    name === 'email'
                      ? 'email'
                      : name === 'phone' || name === 'additional_phone'
                        ? 'tel'
                        : 'text'
                  }
                  value={String(draft[name])}
                  onChange={(e) => setDraft({ ...draft, [name]: e.target.value })}
                />
              </label>
            ))}
            <label>
              Порядок в списке
              <input
                type="number"
                min={0}
                max={10000}
                value={draft.sort_order}
                onChange={(e) => setDraft({ ...draft, sort_order: Number(e.target.value) })}
              />
            </label>
            <label className="workflow-check">
              <input
                type="checkbox"
                checked={draft.emergency}
                onChange={(e) => setDraft({ ...draft, emergency: e.target.checked })}
              />
              Аварийная служба
            </label>
            <div className="workflow-links">
              <button className="primary-btn">Сохранить контакт</button>
              <button className="secondary-btn" type="button" onClick={() => setDraft(undefined)}>
                Отмена
              </button>
            </div>
          </fieldset>
          {validation && <p role="alert">{validation}</p>}
        </form>
      )}
      <QueryState query={q} />
      {q.data?.items.length === 0 && (
        <EmptyState title="Контактов пока нет">
          Председатель может добавить службы вашего дома.
        </EmptyState>
      )}
      {q.data?.items.map((c) => (
        <article key={c.id} className="news-card">
          <span className="category-badge">{contactCategories[c.category]}</span>
          <h2>
            {c.title}
            {!c.is_active ? ' · В архиве' : ''}
          </h2>
          {c.emergency && <strong>Аварийная служба</strong>}
          <p>{c.organization_name}</p>
          <p className="pre-wrap">{c.description}</p>
          <div className="workflow-links">
            <a className="primary-btn" href={'tel:' + c.phone}>
              Позвонить {c.phone}
            </a>
            {c.additional_phone && (
              <a href={'tel:' + c.additional_phone}>Доп. {c.additional_phone}</a>
            )}
            {c.email && <a href={'mailto:' + c.email}>{c.email}</a>}
            {c.website && (
              <a href={c.website} target="_blank" rel="noreferrer">
                Сайт службы
              </a>
            )}
          </div>
          {manage && c.is_active && (
            <div className="workflow-links">
              <button
                className="secondary-btn"
                onClick={() => {
                  setId(c.id);
                  setDraft(contactInputSchema.parse(c));
                  setValidation('');
                }}
              >
                Изменить
              </button>
              <button
                className="secondary-btn"
                disabled={action.isPending}
                onClick={() => setArchive(c.id)}
              >
                В архив
              </button>
            </div>
          )}
        </article>
      ))}
      {archive && (
        <ConfirmDialog
          title="Архивировать контакт?"
          onCancel={() => setArchive('')}
          onConfirm={() => {
            const id = archive;
            setArchive('');
            action.run('archive-' + id, (k) => contactsApi.archive(id, k));
          }}
        >
          Контакт исчезнет из справочника жильцов.
        </ConfirmDialog>
      )}
      {action.error && <ErrorState error={action.error} />}
    </WorkflowFrame>
  );
}
