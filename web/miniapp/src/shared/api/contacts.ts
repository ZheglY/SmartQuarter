import { z } from 'zod';
import { request } from './client';
export const contactCategories = {
  MANAGEMENT_COMPANY: 'Управляющая компания',
  HOA: 'ТСЖ',
  EMERGENCY_DISPATCH: 'Аварийная служба',
  ELECTRICITY: 'Электричество',
  WATER: 'Водоснабжение',
  HEATING: 'Отопление',
  GAS: 'Газ',
  ELEVATOR: 'Лифт',
  WASTE: 'Вывоз отходов',
  INTERNET: 'Интернет',
  SECURITY: 'Охрана',
  OTHER: 'Другое',
} as const;
export const contactInputSchema = z.object({
  category: z.enum(
    Object.keys(contactCategories) as [
      keyof typeof contactCategories,
      ...(keyof typeof contactCategories)[],
    ],
  ),
  title: z.string().min(1).max(150),
  organization_name: z.string().max(255),
  phone: z.string().regex(/^\+[1-9][0-9]{7,14}$/),
  additional_phone: z.union([z.literal(''), z.string().regex(/^\+[1-9][0-9]{7,14}$/)]),
  email: z.union([z.literal(''), z.email()]),
  website: z.union([z.literal(''), z.url().refine((v) => /^https?:\/\//.test(v))]),
  description: z.string().max(2000),
  emergency: z.boolean(),
  sort_order: z.number().int().min(0).max(10000),
});
export const contactSchema = contactInputSchema.extend({
  id: z.string().uuid(),
  house_id: z.string().uuid(),
  is_active: z.boolean(),
  created_by: z.string().optional(),
  created_at: z.string().nullable().optional(),
  updated_at: z.string().nullable().optional(),
});
export type ContactInput = z.infer<typeof contactInputSchema>;
export const contactsApi = {
  list: (archived = false, signal?: AbortSignal) =>
    request(
      '/house/service-contacts' + (archived ? '?include_archived=true' : ''),
      z.object({ items: contactSchema.array() }),
      { signal },
    ),
  create: (body: ContactInput, key: string) =>
    request('/chairman/service-contacts', contactSchema, { method: 'POST', body, key }),
  update: (id: string, body: ContactInput, key: string) =>
    request('/chairman/service-contacts/' + id, contactSchema, { method: 'PATCH', body, key }),
  archive: (id: string, key: string) =>
    request('/chairman/service-contacts/' + id, contactSchema, { method: 'DELETE', body: {}, key }),
};
