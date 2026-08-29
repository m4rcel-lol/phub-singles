import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import { Badge } from '../core/models';

/**
 * The badge row shown next to a display name.
 *
 * Verified carries the orange accent because it is the one visitors are meant
 * to read as a claim about the page; Owner and Administrator are quieter, they
 * describe a role on this site rather than a status.
 */
@Component({
  selector: 'app-badge-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (badge of badges(); track badge.id) {
      <span class="badge" [class]="'badge-' + badge.id" [title]="badge.title">
        @switch (badge.id) {
          @case ('verified') {
            <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
              <path
                d="M8 1.2 9.9 3l2.6-.2.4 2.5 2.1 1.5-1.3 2.2 1.3 2.2-2.1 1.5-.4 2.5-2.6-.2L8 16.3 6.1 14.6l-2.6.2-.4-2.5L1 10.8l1.3-2.2L1 6.4l2.1-1.5.4-2.5 2.6.2z"
                fill="currentColor"
                transform="translate(0 -1.3) scale(0.98)"
              />
              <path
                d="M5.4 8.2 7.2 10l3.4-3.6"
                fill="none"
                stroke="var(--badge-check)"
                stroke-width="1.6"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          }
          @case ('owner') {
            <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
              <path
                d="M8 1.4 13.6 3.6v4.1c0 3.2-2.3 6-5.6 6.9-3.3-.9-5.6-3.7-5.6-6.9V3.6z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.4"
                stroke-linejoin="round"
              />
              <path d="M4.9 7.6 6.9 9.7l4.2-4.3" fill="none" stroke="currentColor"
                stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          }
          @case ('admin') {
            <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
              <path
                d="M8 1.4 13.6 3.6v4.1c0 3.2-2.3 6-5.6 6.9-3.3-.9-5.6-3.7-5.6-6.9V3.6z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.4"
                stroke-linejoin="round"
              />
            </svg>
          }
        }
        <span class="label">{{ badge.label }}</span>
      </span>
    }
  `,
  styles: `
    :host {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }
    .badge {
      /* The tick inside the Verified star is punched out in the badge's own
         background colour, so it stays legible on the orange fill. */
      --badge-check: var(--surface);
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 3px 8px 3px 6px;
      border-radius: 999px;
      border: 1px solid var(--border-strong);
      background: var(--surface);
      color: var(--text-muted);
      font-size: 0.6875rem;
      font-weight: 700;
      letter-spacing: 0.01em;
      line-height: 1.4;
      white-space: nowrap;
      cursor: default;
    }
    .badge svg {
      width: 13px;
      height: 13px;
      flex: none;
    }
    .badge-verified {
      --badge-check: var(--accent);
      background: var(--accent);
      border-color: var(--accent);
      color: var(--accent-ink);
    }
    .badge-owner {
      color: var(--accent);
      border-color: var(--accent-ring);
      background: var(--accent-soft);
    }
  `,
})
export class BadgeList {
  readonly badges = input.required<Badge[]>();
}
