import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

import { AuthService } from '../../core/auth.service';
import { Wordmark } from '../../shared/wordmark';

/** Frame for the signed-in user's own profile, links and account settings. */
@Component({
  selector: 'app-dashboard-shell',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Wordmark],
  templateUrl: './shell.page.html',
  styleUrl: './shell.page.css',
})
export class DashboardShell {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  protected readonly username = this.auth.username;
  protected readonly isAdmin = this.auth.isAdmin;
  protected readonly signingOut = signal(false);
  protected readonly tabs = [
    { path: 'profile', label: 'Profile' },
    { path: 'links', label: 'Links' },
    { path: 'account', label: 'Account' },
  ];

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
