import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { PublicFooter } from '../../shared/public-footer';

/** Shown for unknown paths and for handles that do not exist. */
@Component({
  selector: 'app-not-found',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, PublicFooter],
  template: `
    <main class="doc not-found">
      <p class="code">404</p>
      <h1>Nothing lives here</h1>
      <p class="doc-meta">
        The address you tried does not match any page on this site.
      </p>
      <a class="btn btn-primary" routerLink="/">Back to the start</a>

      <app-public-footer />
    </main>
  `,
  styles: `
    .not-found {
      text-align: center;
      padding-top: 96px;
    }
    .code {
      font-size: 3.5rem;
      font-weight: 800;
      letter-spacing: -0.04em;
      color: var(--accent);
      line-height: 1;
      margin-bottom: 8px;
    }
    .btn {
      margin-top: 8px;
    }
  `,
})
export class NotFoundPage {}
