import { createContext, useContext, useRef, type ReactNode } from 'react';
import type { IssueCategory, CreateIssueRequest } from '../../shared/api/models';
export type PhotoDraft = {
  id: string;
  file: File;
  preview: string;
  stage: 'selected' | 'uploading' | 'validating' | 'ready' | 'failed';
  attachmentId?: string;
};
export type IssueDraft = {
  category: IssueCategory;
  description: string;
  location: string;
  consent: boolean;
  photos: PhotoDraft[];
  attempt?: { key: string; body: CreateIssueRequest };
};
export type FormDraft = {
  title: string;
  body: string;
  attempt?: { key: string; title: string; body: string };
};
export type Drafts = {
  issues: Map<string, IssueDraft>;
  announcements: Map<string, FormDraft>;
  clear: () => void;
};
const Context = createContext<Drafts | null>(null);
export function DraftProvider({ children }: { children: ReactNode }) {
  const store = useRef<Drafts>({
    issues: new Map(),
    announcements: new Map(),
    clear() {
      for (const d of this.issues.values())
        for (const p of d.photos) URL.revokeObjectURL(p.preview);
      this.issues.clear();
      this.announcements.clear();
    },
  });
  return <Context.Provider value={store.current}>{children}</Context.Provider>;
}
export function useDrafts() {
  const v = useContext(Context);
  if (!v) throw new Error('Draft provider required');
  return v;
}
