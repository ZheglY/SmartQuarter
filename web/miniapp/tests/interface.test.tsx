import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { IssueCard } from '../src/shared/ui/components';
import { canManage, byteLength, nextStatuses } from '../src/shared/utils/presentation';
import { contextSchema, detailsSchema } from '../src/shared/api/models';
import { launchIssue, bindBack } from '../src/shared/max/bridge';
import { context, issue, ids } from './fixtures';
describe('contract and interface safeguards', () => {
  it('validates nullable DTO fields and enums', () => {
    expect(contextSchema.safeParse(context).success).toBe(true);
    const details = {
      issue,
      attachments: [],
      confirmed_by_me: false,
      timeline: [],
      latest_statement: null,
    };
    expect(detailsSchema.safeParse(details).success).toBe(true);
    expect(
      detailsSchema.safeParse({ ...details, issue: { ...issue, status: 'INVENTED' } }).success,
    ).toBe(false);
  });
  it('uses the active house membership for role access', () => {
    expect(canManage(context)).toBe(false);
    expect(
      canManage({ ...context, memberships: [{ ...context.memberships[0], role: 'CHAIRMAN' }] }),
    ).toBe(true);
    expect(
      canManage({
        ...context,
        memberships: [{ ...context.memberships[0], role: 'CHAIRMAN', house_id: ids.house2 }],
      }),
    ).toBe(false);
    expect(nextStatuses('RESOLVED', 'CHAIRMAN')).toEqual([]);
    expect(nextStatuses('RESOLVED', 'ADMIN')).toEqual(['CONFIRMING']);
  });
  it('measures UTF8 bytes, renders user content as text', () => {
    expect(byteLength('я')).toBe(2);
    render(
      <MemoryRouter>
        <IssueCard issue={{ ...issue, description: '<img src=x onerror=alert(1)>' }} />
      </MemoryRouter>,
    );
    expect(screen.getByText('<img src=x onerror=alert(1)>')).toBeInTheDocument();
    expect(document.querySelector('img')).toBeNull();
    expect(screen.getByRole('link')).toHaveAttribute('href', '/issues/' + ids.issue);
  });
  it('validates launch payload and cleans bridge subscription', () => {
    expect(launchIssue('start_param=issue_' + ids.issue)).toBe(ids.issue);
    expect(launchIssue('start_param=issue_javascript:alert(1)')).toBeNull();
    const button = { show: vi.fn(), hide: vi.fn(), onClick: vi.fn(), offClick: vi.fn() };
    window.WebApp = { initData: '', BackButton: button };
    const fn = vi.fn(),
      cleanup = bindBack(fn, true);
    expect(button.onClick).toHaveBeenCalledWith(fn);
    cleanup();
    expect(button.offClick).toHaveBeenCalledWith(fn);
    delete window.WebApp;
  });
});
