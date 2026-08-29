import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';

import { ApiService, apiErrorCode, apiMessage } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { PagePayload } from '../../core/models';
import { BadgeList } from '../../shared/badge-list';
import { MaintenanceNotice } from '../../shared/maintenance-notice';
import { PublicFooter } from '../../shared/public-footer';
import { Wordmark } from '../../shared/wordmark';

/**
 * The site's front door. It explains what the project is and points at the
 * published profile, which lives at /<handle>.
 */
@Component({
  selector: 'app-landing',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink, BadgeList, MaintenanceNotice, Wordmark, PublicFooter],
  templateUrl: './landing.page.html',
  styleUrl: './landing.page.css',
})
export class LandingPage implements OnInit {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);

  protected readonly page = signal<PagePayload | null>(null);
  protected readonly signingOut = signal(false);
  protected readonly maintenance = signal<string | null>(null);

  /** Owner-controlled copy, with the shipped defaults as a fallback. */
  protected readonly headline = computed(() => this.page()?.site.headline ?? 'Every link. One page.');
  protected readonly lede = computed(
    () =>
      this.page()?.site.lede ??
      'A single, fast, self-hosted bio page. No feeds, no trackers, no clutter — just the links that matter, in the order you choose.',
  );

  /** Drives the auth buttons in the header. */
  protected readonly isAdmin = this.auth.isAdmin;
  protected readonly username = this.auth.username;
  protected readonly sessionResolved = this.auth.resolved;

  /** Route to the published profile, once the handle is known. */
  protected readonly profilePath = computed(() => {
    const handle = this.page()?.profile.username;
    return handle ? `/${handle}` : null;
  });

  protected readonly linkCount = computed(() => this.page()?.links.length ?? 0);

  ngOnInit(): void {
    // A failure here is not fatal: the page still renders without the CTA.
    this.api.page().subscribe({
      next: (page) => this.page.set(page),
      error: (err: unknown) => {
        this.page.set(null);
        if (apiErrorCode(err) === 'maintenance') {
          this.maintenance.set(apiMessage(err, 'Back shortly.'));
        }
      },
    });
    // Public endpoint: anonymous visitors get {authenticated:false}, not a 401.
    this.auth.ensureSession().subscribe();
  }

  protected signOut(): void {
    if (this.signingOut()) {
      return;
    }
    this.signingOut.set(true);
    this.auth.logout().subscribe(() => this.signingOut.set(false));
  }
}
