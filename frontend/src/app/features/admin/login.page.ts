import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';

import { apiMessage } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { Wordmark } from '../../shared/wordmark';

@Component({
  selector: 'app-login',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, Wordmark],
  templateUrl: './login.page.html',
  styleUrl: './login.page.css',
})
export class LoginPage {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly username = signal('');
  protected readonly password = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);

  /**
   * Where to go after signing in. The guard puts the attempted URL in `next`;
   * only in-app admin paths are honoured, so the parameter cannot be used to
   * bounce someone off the site.
   */
  private destination(): string {
    const next = this.route.snapshot.queryParamMap.get('next') ?? '';
    return next.startsWith('/admin') && !next.startsWith('//') ? next : '/admin/links';
  }

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    const username = this.username().trim();
    const password = this.password();
    if (!username || !password) {
      this.error.set('Enter your username and password.');
      return;
    }

    this.busy.set(true);
    this.error.set(null);

    this.auth.login(username, password).subscribe({
      next: () => {
        this.busy.set(false);
        void this.router.navigateByUrl(this.destination());
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.password.set('');
        this.error.set(apiMessage(err, 'Sign in failed.'));
      },
    });
  }
}
