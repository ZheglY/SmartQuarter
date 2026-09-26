import { z } from 'zod';
import { request } from './client';
const id = z.string().uuid();
const date = z.string().datetime({ offset: true });
export const PollSchema = z.object({
  id,
  question: z.string(),
  status: z.enum(['POLL_STATUS_OPEN', 'POLL_STATUS_CLOSED']),
  ends_at: date,
  options: z.array(z.object({ id, text: z.string(), position: z.number() })),
});
export const PollDetailsSchema = z.object({
  poll: PollSchema,
  total_votes: z.number(),
  my_option_id: z.string(),
  results: z.array(z.object({ option_id: id, votes_count: z.number() })),
});
export const CalendarSchema = z.object({
  id,
  title: z.string(),
  description: z.string(),
  starts_at: date,
  ends_at: date,
});
export const InitiativeSchema = z.object({
  id,
  title: z.string(),
  description: z.string(),
  status: z.enum(['INITIATIVE_STATUS_OPEN', 'INITIATIVE_STATUS_CLOSED']),
  supports_count: z.number(),
  supported_by_me: z.boolean(),
});
export type CalendarEvent = z.infer<typeof CalendarSchema>;
export type CalendarInput = Omit<CalendarEvent, 'id'>;
const page = <T extends z.ZodType>(item: T) =>
  z.object({ items: z.array(item), next_page_token: z.string() });
export const communityApi = {
  polls: (status: string, token: string, signal?: AbortSignal) =>
    request(
      `/polls?status=${status}&page_size=20&page_token=${encodeURIComponent(token)}`,
      page(PollSchema),
      { signal },
    ),
  poll: (id: string, signal?: AbortSignal) =>
    request(`/polls/${id}`, PollDetailsSchema, { signal }),
  createPoll: (body: { question: string; options: string[]; ends_at: string }, key: string) =>
    request('/polls', PollSchema, { method: 'POST', body, key }),
  vote: (id: string, option_id: string, key: string) =>
    request(`/polls/${id}/vote`, z.object({ total_votes: z.number() }), {
      method: 'POST',
      body: { option_id },
      key,
    }),
  closePoll: (id: string, key: string) =>
    request(`/polls/${id}/close`, PollDetailsSchema, { method: 'POST', body: {}, key }),
  calendar: (from: string, to: string, signal?: AbortSignal) =>
    request(
      `/calendar?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
      z.object({ items: z.array(CalendarSchema) }),
      { signal },
    ),
  saveEvent: (id: string, body: CalendarInput, key: string) =>
    request(`/calendar${id ? '/' + id : ''}`, CalendarSchema, {
      method: id ? 'PATCH' : 'POST',
      body,
      key,
    }),
  deleteEvent: (id: string, key: string) =>
    request(`/calendar/${id}`, z.object({ id: z.string().uuid() }), {
      method: 'DELETE',
      body: {},
      key,
    }),
  initiatives: (token: string, signal?: AbortSignal) =>
    request(
      `/initiatives?page_size=20&page_token=${encodeURIComponent(token)}`,
      page(InitiativeSchema),
      { signal },
    ),
  createInitiative: (body: { title: string; description: string }, key: string) =>
    request('/initiatives', InitiativeSchema, { method: 'POST', body, key }),
  support: (id: string, key: string) =>
    request(
      `/initiatives/${id}/support`,
      z.object({ supports_count: z.number(), supported_by_me: z.boolean() }),
      { method: 'POST', body: {}, key },
    ),
  closeInitiative: (id: string, key: string) =>
    request(`/initiatives/${id}/close`, InitiativeSchema, { method: 'POST', body: {}, key }),
};
