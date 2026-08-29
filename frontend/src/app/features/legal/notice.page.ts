import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { PublicFooter } from '../../shared/public-footer';

/**
 * The parody notice. It exists so nobody has to guess what this site is: a
 * humour project that borrows a very recognisable colour scheme.
 */
@Component({
  selector: 'app-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, PublicFooter],
  template: `
    <main class="doc">
      <a class="doc-back" routerLink="/">← Home</a>

      <h1>Notice</h1>
      <p class="doc-meta">The short version: this is a joke, and nothing here is official.</p>

      <div class="doc-callout">
        <p>
          <strong>pornhub.singles is a humour project.</strong> It is not affiliated with,
          endorsed by, sponsored by or connected to Pornhub, Aylo, or any of their companies,
          brands or services. The name is a pun on a domain ending, nothing more.
        </p>
      </div>

      <h2>What this site actually is</h2>
      <p>
        A small, self-hosted bio-link page — the kind of page that holds someone's links so they
        can share one address instead of ten. The dark theme and the orange accent are a knowing
        wink at a famous logo. That wink is the entire joke.
      </p>

      <h2>What it is not</h2>
      <ul>
        <li>It is not a tube site, and it hosts no adult media of any kind.</li>
        <li>It is not operated by, or on behalf of, any adult entertainment company.</li>
        <li>It does not claim any rights in anyone else's trademarks, logos or branding.</li>
      </ul>

      <h2>Trademarks</h2>
      <p>
        All trademarks, service marks and trade names referenced on this site remain the property
        of their respective owners. They are used here, if at all, only to describe the joke being
        made — never to suggest an association.
      </p>

      <h2>Links go elsewhere</h2>
      <p>
        Every button on a profile page points at a third-party website. Those sites are outside
        this project's control; their content, terms and privacy practices are their own.
      </p>

      <h2>Rights holders and complaints</h2>
      <p>
        If you represent a rights holder and something here needs to change, write to
        <a href="mailto:hello&#64;pornhub.singles">hello&#64;pornhub.singles</a>. Real people read
        that inbox and would rather fix a problem than argue about it.
      </p>

      <app-public-footer />
    </main>
  `,
})
export class NoticePage {}
