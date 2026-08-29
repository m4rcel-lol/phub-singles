import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * The site wordmark: "porn" in white, "hub" boxed in the signature orange,
 * ".singles" muted. Used in the public footer and the admin header.
 */
@Component({
  selector: 'app-wordmark',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="wordmark" [class.small]="small()">
      <span class="w-porn">porn</span><span class="w-hub">hub</span
      ><span class="w-tld">.singles</span>
    </span>
  `,
  styles: `
    .wordmark {
      display: inline-flex;
      align-items: center;
      font-weight: 800;
      font-size: 1.0625rem;
      letter-spacing: -0.02em;
      line-height: 1;
      white-space: nowrap;
    }
    .wordmark.small {
      font-size: 0.875rem;
    }
    .w-porn {
      color: var(--text);
    }
    .w-hub {
      background: var(--accent);
      color: var(--accent-ink);
      padding: 3px 5px 4px;
      border-radius: 4px;
      margin: 0 1px;
    }
    .w-tld {
      color: var(--text-faint);
      font-weight: 600;
      margin-left: 3px;
    }
  `,
})
export class Wordmark {
  /** Renders a compact variant for dense layouts. */
  readonly small = input(false);
}
