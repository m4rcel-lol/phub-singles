import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { Wordmark } from './wordmark';

/** Footer shared by every public page: wordmark, legal links, copyright. */
@Component({
  selector: 'app-public-footer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, Wordmark],
  template: `
    <footer class="footer">
      <a routerLink="/" class="mark" aria-label="Home">
        <app-wordmark [small]="true" />
      </a>

      <nav class="links" aria-label="Site information">
        <a routerLink="/notice">Notice</a>
        <a routerLink="/privacy">Privacy</a>
        <a routerLink="/terms">Terms</a>
      </nav>

      <p class="copy">© {{ year }} pornhub.singles · a parody project</p>
    </footer>
  `,
  styles: `
    .footer {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 12px;
      padding: 40px 0 8px;
      text-align: center;
    }
    .links {
      display: flex;
      gap: 18px;
    }
    .links a {
      font-size: 0.8125rem;
      color: var(--text-muted);
      transition: color 140ms ease;
    }
    .links a:hover {
      color: var(--accent);
    }
    .copy {
      font-size: 0.75rem;
      color: var(--text-faint);
    }
  `,
})
export class PublicFooter {
  protected readonly year = new Date().getFullYear();
}
