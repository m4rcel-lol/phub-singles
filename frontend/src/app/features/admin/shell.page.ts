import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

import { AuthService } from '../../core/auth.service';
import { Wordmark } from '../../shared/wordmark';

/** Frame around every admin screen: brand, navigation and sign-out. */
@Component({
  selector: 'app-admin-shell',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Wordmark],
  templateUrl: './shell.page.html',
  styleUrl: './shell.page.css',
})
export class AdminShell {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  protected readonly username = this.auth.username;
  protected readonly signingOut = signal(false);

  /** The Site tab is the owner panel, so it only appears for the owner. */
  protected readonly tabs = computed(() => [
    { path: 'links', label: 'Links' },
    { path: 'profile', label: 'Profile' },
    { path: 'stats', label: 'Stats' },
    { path: 'users', label: 'People' },
    ...(this.auth.role() === 'owner' ? [{ path: 'site', label: 'Site' }] : []),
    { path: 'account', label: 'Account' },
  ]);

  protected readonly role = this.auth.role;

  protected signOut(): void {
    if (this.signingOut()) {
      return;
    }
    this.signingOut.set(true);
    this.auth.logout().subscribe(() => {
      this.signingOut.set(false);
      void this.router.navigate(['/admin/login']);
    });
  }
}
