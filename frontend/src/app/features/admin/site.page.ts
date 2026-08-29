import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiService, apiMessage } from '../../core/api.service';
import { SettingsPayload, SiteSettings } from '../../core/models';

/**
 * The owner panel: settings that change what every visitor sees.
 *
 * These are deliberately not environment variables — an owner should be able to
 * put the site into maintenance, or move the verification threshold, without
 * editing .env and redeploying.
 */
@Component({
  selector: 'app-admin-site',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  templateUrl: './site.page.html',
  styleUrl: './site.page.css',
})
export class SitePage implements OnInit {
  private readonly api = inject(ApiService);

  protected readonly settings = signal<SiteSettings | null>(null);
  protected readonly limits = signal<SettingsPayload['limits'] | null>(null);

  protected readonly loading = signal(true);
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);

  ngOnInit(): void {
    this.api.settings().subscribe({
      next: (payload) => {
        this.settings.set(payload.settings);
        this.limits.set(payload.limits);
        this.loading.set(false);
      },
      error: (err: unknown) => {
        this.error.set(apiMessage(err, 'Could not load the settings.'));
        this.loading.set(false);
      },
    });
  }

  protected patch(patch: Partial<SiteSettings>): void {
    this.settings.update((current) => (current ? { ...current, ...patch } : current));
  }

  protected save(): void {
    const settings = this.settings();
    if (!settings || this.saving()) {
      return;
    }

    this.saving.set(true);
    this.error.set(null);

    this.api.updateSettings(settings).subscribe({
      next: (payload) => {
        this.saving.set(false);
        this.settings.set(payload.settings);
        this.flash('Settings saved.');
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.error.set(apiMessage(err, 'Could not save the settings.'));
      },
    });
  }

  private flash(message: string): void {
    this.notice.set(message);
    setTimeout(() => this.notice.set(null), 2500);
  }
}
