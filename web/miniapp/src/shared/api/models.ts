import { z } from 'zod';
export const categories = [
  'SAFETY',
  'CLEANLINESS',
  'UTILITIES',
  'INFRASTRUCTURE',
  'OTHER',
] as const;
export const statuses = [
  'DETECTED',
  'CONFIRMING',
  'READY_FOR_APPEAL',
  'HANDED_TO_CHAIRMAN',
  'MARKED_SENT',
  'WAITING_RESULT',
  'RESOLVED',
] as const;
export const roles = ['RESIDENT', 'CHAIRMAN', 'ADMIN'] as const;
const id = z.string().uuid();
const date = z.string().datetime({ offset: true });
export const userSchema = z.object({
  id,
  max_user_id: z.string(),
  display_name: z.string(),
  username: z.string(),
  created_at: date,
  updated_at: date,
});
export const houseSchema = z.object({
  id,
  name: z.string(),
  address: z.string(),
  city: z.string(),
});
export const membershipSchema = z.object({
  id,
  user_id: id,
  house_id: id,
  role: z.enum(roles),
  status: z.enum(['ACTIVE', 'INACTIVE']),
});
export const contextSchema = z.object({
  user: userSchema,
  houses: z.array(houseSchema),
  memberships: z.array(membershipSchema),
  default_house_id: z.string(),
  active_house_id: z.string(),
});
export const issueSchema = z.object({
  id,
  house_id: id,
  created_by: id,
  house_address_snapshot: z.string(),
  category: z.enum(categories),
  description: z.string(),
  location_text: z.string(),
  status: z.enum(statuses),
  confirmations_count: z.number().int(),
  created_at: date,
  updated_at: date,
  resolved_at: date.nullable(),
});
export const attachmentSchema = z.object({
  id,
  house_id: id,
  issue_id: id.nullable(),
  uploaded_by: id,
  original_filename: z.string(),
  mime_type: z.string(),
  size_bytes: z.number(),
  sha256: z.string(),
  etag: z.string(),
  status: z.enum(['UPLOADING', 'READY', 'ATTACHED', 'REJECTED', 'EXPIRED']),
  upload_expires_at: date,
  created_at: date,
  updated_at: date,
});
export const statementSchema = z.object({
  id,
  issue_id: id,
  version: z.number().int(),
  status: z.string(),
  body: z.string(),
  chairman_note: z.string(),
  source_snapshot: z.record(z.string(), z.unknown()),
  created_by: id,
  created_at: date,
  updated_at: date,
});
export const timelineSchema = z.object({
  id,
  issue_id: id,
  type: z.string(),
  actor_user_id: id,
  payload: z.record(z.string(), z.unknown()),
  created_at: date,
});
export const detailsSchema = z.object({
  issue: issueSchema,
  attachments: z.array(attachmentSchema),
  confirmed_by_me: z.boolean(),
  timeline: z.array(timelineSchema),
  latest_statement: statementSchema.nullable(),
});
export const announcementSchema = z.object({
  id,
  house_id: id,
  author_user_id: id,
  title: z.string(),
  body: z.string(),
  status: z.literal('PUBLISHED'),
  published_at: date,
  created_at: date,
});
export const uploadSchema = z.object({
  upload_id: id,
  presigned_url: z.string().url(),
  expires_at: date,
  required_headers: z.record(z.string(), z.string()),
});
export const downloadSchema = z.object({ url: z.string().url(), expires_at: date });
export const pageSchema = <T extends z.ZodType>(item: T) =>
  z.object({ items: z.array(item), next_page_token: z.string() });
export type User = z.infer<typeof userSchema>;
export type House = z.infer<typeof houseSchema>;
export type Membership = z.infer<typeof membershipSchema>;
export type UserContext = z.infer<typeof contextSchema>;
export type Issue = z.infer<typeof issueSchema>;
export type IssueDetails = z.infer<typeof detailsSchema>;
export type Attachment = z.infer<typeof attachmentSchema>;
export type StatementDraft = z.infer<typeof statementSchema>;
export type TimelineEvent = z.infer<typeof timelineSchema>;
export type Announcement = z.infer<typeof announcementSchema>;
export type IssueCategory = Issue['category'];
export type IssueStatus = Issue['status'];
export type Role = Membership['role'];
export type PaginatedResponse<T> = { items: T[]; next_page_token: string };
export type IssueListResponse = PaginatedResponse<Issue>;
export type AnnouncementListResponse = PaginatedResponse<Announcement>;
export type CreateUploadRequest = { filename: string; mime_type: string; size_bytes: number };
export type CreateUploadResponse = z.infer<typeof uploadSchema>;
export type CreateIssueRequest = {
  category: IssueCategory;
  description: string;
  location_text: string;
  attachment_ids: string[];
};
