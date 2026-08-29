import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink } from '@angular/router';

import { PublicFooter } from '../../shared/public-footer';

/**
 * Privacy policy. The text deliberately describes exactly what the code does:
 * two counters, one admin cookie, no third parties.
 */
@Component({
  selector: 'app-privacy',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, PublicFooter],
  template: `
    <main class="doc">
      <a class="doc-back" routerLink="/">← Home</a>

      <h1>Privacy policy</h1>
      <p class="doc-meta">Last updated {{ updated }}</p>

      <p>
        This site collects as little as it can while still being able to tell whether anyone is
        looking at it. There are no third-party analytics, no advertising networks, no social
        widgets and no tracking pixels. Nothing is sold or shared for marketing.
      </p>

      <h2>What is counted</h2>
      <ul>
        <li><strong>Page views:</strong> a single number that goes up by one when a profile page loads.</li>
        <li><strong>Link clicks:</strong> a per-link counter that goes up by one when a link is opened.</li>
      </ul>
      <p>
        Both are plain integers in a local SQLite database. Neither is linked to a visitor, a
        session or an account, so the numbers cannot be traced back to anyone.
      </p>

      <h2>Counting each visitor once</h2>
      <p>
        For those numbers to mean anything, the site has to tell a repeat visit apart from a new
        one. It does that with a short-lived fingerprint: a keyed hash of your IP address and
        browser user agent, computed under a secret that never leaves the server and truncated to
        128 bits. It cannot be reversed into an address, it is never stored alongside the counters
        or anything else about you, and it is deleted when it expires — within 12 hours for a page
        view, 15 minutes for a link click. While it exists, further visits from you are simply not
        counted.
      </p>
      <p>
        Requests that did not come from a page on this site, and requests from bots and
        command-line clients, are ignored before any of this happens.
      </p>

      <h2>Cookies</h2>
      <p>
        Visitors get no cookies at all. Signing in to the admin panel sets exactly one cookie,
        <code>phs_session</code>, which holds a random session token. It is HttpOnly,
        SameSite=Strict, sent only to this site's API, and it is deleted on sign-out.
      </p>

      <h2>Server logs</h2>
      <p>
        The server writes one structured log line per request: timestamp, method, path, status
        code, response size, duration and the requesting IP address. IP addresses are also held
        in memory for a short while to enforce rate limits on the public write endpoints. Logs
        are kept only as long as the hosting configuration retains them and are used for
        debugging and abuse prevention, nothing else.
      </p>

      <h2>Uploads</h2>
      <p>
        The only file this site stores is the avatar image uploaded by the page owner. It is
        served from this domain and is not sent anywhere else.
      </p>

      <h2>Third-party links</h2>
      <p>
        Opening a link leaves this site. Whatever the destination collects is governed by its own
        privacy policy, not this one.
      </p>

      <h2>Your requests</h2>
      <p>
        Because no personal data is stored about visitors, there is normally nothing to export or
        delete. If you believe otherwise, or you want a log entry removed, write to
        <a href="mailto:hello&#64;pornhub.singles">hello&#64;pornhub.singles</a>.
      </p>

      <h2>Changes</h2>
      <p>
        If this policy changes, the date at the top changes with it. Continued use after that
        means the new version applies.
      </p>

      <app-public-footer />
    </main>
  `,
})
export class PrivacyPage {
  protected readonly updated = '29 August 2026';
}
