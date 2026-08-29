import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { PublicFooter } from '../../shared/public-footer';

@Component({
  selector: 'app-terms',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, PublicFooter],
  template: `
    <main class="doc">
      <a class="doc-back" routerLink="/">← Home</a>

      <h1>Terms of service</h1>
      <p class="doc-meta">Last updated {{ updated }}</p>

      <p>
        By using this site you agree to what follows. If you do not, please close the tab — no
        hard feelings.
      </p>

      <h2>1. What this is</h2>
      <p>
        pornhub.singles is a personal bio-link page and a parody project. It is not affiliated
        with any adult entertainment company; see the <a routerLink="/notice">notice</a> for the
        full explanation.
      </p>

      <h2>2. Who may use it</h2>
      <p>
        The site is intended for adults (18 or older, or the age of majority where you live). It
        hosts no adult media itself, but the links it points to are outside this site's control.
      </p>

      <h2>3. Acceptable use</h2>
      <ul>
        <li>Do not attempt to gain access to the admin panel or any account you do not own.</li>
        <li>Do not scrape, flood or otherwise automate requests beyond ordinary browsing. Public
          endpoints are rate limited and abusive traffic is blocked.</li>
        <li>Do not attempt to disrupt, probe or reverse the service, or to interfere with anyone
          else's use of it.</li>
      </ul>

      <h2>4. Links to other sites</h2>
      <p>
        Every link opens a third-party website. Those sites are not operated by this project, and
        their content, availability, terms and privacy practices are their own responsibility.
        Listing a link is not an endorsement.
      </p>

      <h2>5. Content and intellectual property</h2>
      <p>
        The page owner is responsible for the profile text, avatar and links published here.
        Trademarks and other rights referenced anywhere on the site belong to their owners. If
        something published here infringes your rights, write to
        <a href="mailto:hello&#64;pornhub.singles">hello&#64;pornhub.singles</a> with the details
        and it will be reviewed promptly.
      </p>

      <h2>6. Availability</h2>
      <p>
        The service is provided on an "as is" and "as available" basis, with no warranty of any
        kind. It may be changed, interrupted or discontinued at any time, without notice.
      </p>

      <h2>7. Liability</h2>
      <p>
        To the fullest extent permitted by applicable law, the operator is not liable for any
        indirect, incidental or consequential damages arising from the use of, or inability to
        use, this site or any site it links to.
      </p>

      <h2>8. Changes</h2>
      <p>
        These terms may be updated; the date above will change when they are. Continuing to use
        the site after an update means you accept the current version.
      </p>

      <h2>9. Contact</h2>
      <p>
        Questions go to <a href="mailto:hello&#64;pornhub.singles">hello&#64;pornhub.singles</a>.
      </p>

      <app-public-footer />
    </main>
  `,
})
export class TermsPage {
  protected readonly updated = '29 August 2026';
}
