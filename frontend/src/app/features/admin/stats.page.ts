import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';

import { ApiService, apiMessage } from '../../core/api.service';
import { Stats } from '../../core/models';

@Component({
  selector: 'app-admin-stats',
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './stats.page.html',
  styleUrl: './stats.page.css',
})
export class StatsPage implements OnInit {
  private readonly api = inject(ApiService);

  protected readonly stats = signal<Stats | null>(null);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly days = signal(14);

  /** Links sorted by clicks, so the table reads as a ranking. */
  protected readonly ranked = computed(() =>
    [...(this.stats()?.perLink ?? [])].sort((a, b) => b.clicks - a.clicks),
  );

  /** Largest click count, used to scale the bars. */
  protected readonly peakClicks = computed(() =>
    Math.max(1, ...this.ranked().map((link) => link.clicks)),
  );

  /** Largest daily value, used to scale the activity chart. */
  protected readonly peakDaily = computed(() =>
    Math.max(1, ...(this.stats()?.daily ?? []).map((d) => Math.max(d.views, d.clicks))),
  );

  /** Click-through rate across the whole page. */
  protected readonly ctr = computed(() => {
    const s = this.stats();
    if (!s || s.totalViews === 0) {
      return '—';
    }
    return `${Math.round((s.totalClicks / s.totalViews) * 100)}%`;
  });

  ngOnInit(): void {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.error.set(null);

    this.api.stats(this.days()).subscribe({
      next: (stats) => {
        this.stats.set(stats);
        this.loading.set(false);
      },
      error: (err: unknown) => {
        this.error.set(apiMessage(err, 'Could not load stats.'));
        this.loading.set(false);
      },
    });
  }

  protected setRange(days: number): void {
    if (this.days() === days) {
      return;
    }
    this.days.set(days);
    this.load();
  }

  /** Bar width as a percentage of the busiest link. */
  protected share(clicks: number): number {
    return Math.round((clicks / this.peakClicks()) * 100);
  }

  /** Column height as a percentage of the busiest day. */
  protected height(value: number): number {
    return Math.max(value === 0 ? 0 : 4, Math.round((value / this.peakDaily()) * 100));
  }

  /** "2026-08-29" → "29 Aug", for the chart axis. */
  protected shortDay(day: string): string {
    const date = new Date(`${day}T00:00:00Z`);
    return Number.isNaN(date.getTime())
      ? day
      : date.toLocaleDateString(undefined, { day: 'numeric', month: 'short', timeZone: 'UTC' });
  }
}
