import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { Title } from '@angular/platform-browser';

import { ApiService, apiErrorCode, apiMessage } from '../../core/api.service';
import { PagePayload, PublicLink } from '../../core/models';
import { BadgeList } from '../../shared/badge-list';
import { MaintenanceNotice } from '../../shared/maintenance-notice';
import { VerifiedMark } from '../../shared/verified-mark';
import { PublicFooter } from '../../shared/public-footer';

/**
 * The public bio page, served at /<handle>: profile header plus the ordered
 * list of links. Everything it needs arrives in a single request.
 */
@Component({
  selector: 'app-home',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [BadgeList, MaintenanceNotice, PublicFooter, VerifiedMark],
  templateUrl: './home.page.html',
  styleUrl: './home.page.css',
})
export class HomePage {
  private readonly api = inject(ApiService);
  private readonly title = inject(Title);

  /** Bound from the :handle route parameter. */
  readonly handle = input('');

  protected readonly page = signal<PagePayload | null>(null);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly missing = signal(false);
  /** Non-null while the owner has the site in maintenance mode. */
  protected readonly maintenance = signal<string | null>(null);

  /**
   * The Verified badge is pulled out of the badge row and rendered as a seal
   * next to the name; the remaining badges (Owner, Administrator) stay as
   * labelled chips, where the label is what carries the meaning.
   */
  protected readonly verified = computed(
    () => this.page()?.badges.find((badge) => badge.id === 'verified') ?? null,
  );

  protected readonly roleBadges = computed(
    () => this.page()?.badges.filter((badge) => badge.id !== 'verified') ?? [],
  );

  /** Fallback avatar: the first character of the display name. */
  protected readonly initial = computed(() => {
    const name = this.page()?.profile.displayName?.trim() ?? '';
    return name ? [...name][0].toUpperCase() : '•';
  });

  constructor() {
    // Reloads if the router reuses this component for a different handle.
    effect(() => this.load(this.handle()));
  }

  protected reload(): void {
    this.load(this.handle());
  }

  private load(handle: string): void {
    this.loading.set(true);
    this.error.set(null);
    this.missing.set(false);
    this.maintenance.set(null);

    this.api.page(handle || undefined).subscribe({
      next: (page) => {
        this.page.set(page);
        this.loading.set(false);
        this.title.setTitle(`${page.profile.displayName} · pornhub.singles`);
        // Only a real page view is counted, never a 404.
        this.api.registerView();
      },
      error: (err: unknown) => {
        this.loading.set(false);
        if (err instanceof HttpErrorResponse && err.status === 404) {
          this.missing.set(true);
          this.title.setTitle('Not found · pornhub.singles');
          return;
        }
        if (apiErrorCode(err) === 'maintenance') {
          this.maintenance.set(apiMessage(err, 'Back shortly.'));
          this.title.setTitle('Maintenance · pornhub.singles');
          return;
        }
        this.error.set(apiMessage(err, 'This page could not be loaded.'));
      },
    });
  }

  /**
   * Counts the click without delaying navigation: the anchor keeps its native
   * behaviour (new tab, middle-click, "open in new window") and the beacon is
   * sent alongside it.
   */
  protected onLinkClick(link: PublicLink): void {
    this.api.registerClick(link.id);
  }
}
