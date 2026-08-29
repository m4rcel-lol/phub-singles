import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * The Verified seal shown next to a display name.
 *
 * On the public page the check sits beside the name at reading size rather than
 * hiding in a chip below it — that is the arrangement people already recognise.
 *
 * The scallop is a nine-lobed star drawn from exact coordinates and then both
 * filled and stroked with round joins, which rounds the points off. Freehand
 * paths at this size read as ragged rather than as a seal.
 */
@Component({
  selector: 'app-verified-mark',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="0 0 24 24" role="img" aria-label="Verified">
      <title>{{ title() }}</title>
      <path
        d="M12.00 2.50 L14.57 4.95 L18.11 4.72 L18.50 8.25 L21.36 10.35 L19.39 13.30 L20.23 16.75 L16.82 17.75 L15.25 20.93 L12.00 19.50 L8.75 20.93 L7.18 17.75 L3.77 16.75 L4.61 13.30 L2.64 10.35 L5.50 8.25 L5.89 4.72 L9.43 4.95 Z"
        fill="currentColor"
        stroke="currentColor"
        stroke-width="2.2"
        stroke-linejoin="round"
      />
      <path
        d="M8.4 12.2l2.5 2.5 4.7-5.1"
        fill="none"
        stroke="var(--verified-tick, var(--bg))"
        stroke-width="2.2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
  `,
  styles: `
    :host {
      display: inline-flex;
      align-items: center;
      color: var(--accent);
      /* Sized against the text it sits beside, so it scales with the name. */
      font-size: inherit;
      line-height: 1;
    }
    svg {
      width: 0.8em;
      height: 0.8em;
      flex: none;
      display: block;
      overflow: visible;
    }
  `,
})
export class VerifiedMark {
  /** Tooltip explaining why the seal is there (granted, or unlocked by views). */
  readonly title = input('Verified');
}
