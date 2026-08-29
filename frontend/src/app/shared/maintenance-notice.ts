import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import { PublicFooter } from './public-footer';
import { Wordmark } from './wordmark';

/**
 * Shown to visitors while the owner has maintenance mode on. Administrators
 * never see it — the API keeps serving them the real page so they can check
 * their work while the site is dark.
 */
@Component({
  selector: 'app-maintenance-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [PublicFooter, Wordmark],
  template: `
    <main class="notice">
      <app-wordmark />
      <p class="badge">Maintenance</p>
      <p class="message">{{ message() }}</p>
      <app-public-footer />
    </main>
  `,
  styles: `
    .notice {
      width: 100%;
      max-width: 480px;
      margin: 0 auto;
      padding: 96px 20px 40px;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 14px;
      text-align: center;
    }
    .badge {
      padding: 3px 10px;
      border-radius: 999px;
      background: var(--accent-soft);
      border: 1px solid var(--accent);
      color: var(--accent);
      font-size: 0.6875rem;
      font-weight: 700;
      letter-spacing: 0.06em;
      text-transform: uppercase;
    }
    .message {
      color: var(--text-muted);
      font-size: 1rem;
      text-wrap: pretty;
    }
  `,
})
export class MaintenanceNotice {
  readonly message = input('We are doing a bit of maintenance. Back shortly.');
}
