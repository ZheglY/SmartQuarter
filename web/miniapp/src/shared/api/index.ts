import { z } from 'zod';
import { request } from './client';
import {
  contextSchema,
  issueSchema,
  detailsSchema,
  attachmentSchema,
  statementSchema,
  announcementSchema,
  pageSchema,
  uploadSchema,
  downloadSchema,
  type CreateUploadRequest,
  type CreateIssueRequest,
  type IssueStatus,
} from './models';
const empty = z.void();
const query = (pageToken = '', status: IssueStatus[] = [], size = 20) => {
  const q = new URLSearchParams({ page_size: String(size) });
  if (pageToken) q.set('page_token', pageToken);
  for (const s of status) q.append('status', s);
  return '?' + q;
};
export const api = {
  bootstrap: (init_data: string, signal?: AbortSignal) =>
    request('/session/max', z.object({ user_context: contextSchema, expires_at: z.string() }), {
      method: 'POST',
      body: { init_data },
      signal,
      bootstrap: true,
    }),
  me: (signal?: AbortSignal) => request('/me', contextSchema, { signal }),
  switchHouse: (house_id: string) =>
    request(
      '/session/active-house',
      z.object({ active_house_id: z.string(), role: z.enum(['RESIDENT', 'CHAIRMAN', 'ADMIN']) }),
      { method: 'POST', body: { house_id } },
    ),
  logout: () => request('/session/logout', empty, { method: 'POST', body: {} }),
  issues: (
    houseQueue = false,
    token = '',
    status: IssueStatus[] = [],
    signal?: AbortSignal,
    size = 20,
  ) =>
    request(
      (houseQueue ? '/chairman/issues' : '/issues') + query(token, status, size),
      pageSchema(issueSchema),
      { signal },
    ),
  issue: (id: string, signal?: AbortSignal) => request('/issues/' + id, detailsSchema, { signal }),
  createUpload: (body: CreateUploadRequest, signal?: AbortSignal) =>
    request('/uploads', uploadSchema, { method: 'POST', body, signal }),
  completeUpload: (id: string, signal?: AbortSignal) =>
    request('/uploads/' + id + '/complete', attachmentSchema, { method: 'POST', body: {}, signal }),
  download: (id: string, signal?: AbortSignal) =>
    request('/attachments/' + id + '/download-url', downloadSchema, { signal }),
  createIssue: (body: CreateIssueRequest, key: string, signal?: AbortSignal) =>
    request('/issues', issueSchema, { method: 'POST', body, key, signal }),
  confirm: (id: string) =>
    request(
      '/issues/' + id + '/confirm',
      z.object({ confirmation_count: z.number(), confirmed_by_me: z.boolean() }),
      { method: 'POST', body: {} },
    ),
  statement: (id: string, signal?: AbortSignal) =>
    request('/issues/' + id + '/statement', statementSchema, { signal }),
  generate: (id: string, chairman_note: string, key: string) =>
    request('/issues/' + id + '/statement', statementSchema, {
      method: 'POST',
      body: { chairman_note },
      key,
    }),
  status: (id: string, new_status: IssueStatus) =>
    request('/issues/' + id + '/status', issueSchema, { method: 'PATCH', body: { new_status } }),
  announcements: (token = '', signal?: AbortSignal, size = 20) =>
    request('/announcements' + query(token, [], size), pageSchema(announcementSchema), { signal }),
  announce: (title: string, body: string, key: string) =>
    request('/announcements', announcementSchema, { method: 'POST', body: { title, body }, key }),
};
