import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';

import { apiFieldErrors, apiMessage } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { Wordmark } from '../../shared/wordmark';

/** Public, password-only registration. Email is intentionally optional. */
@Component({
  selector: 'app-register',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, RouterLink, Wordmark],
  templateUrl: './register.page.html',
  styleUrl: './register.page.css',
})
export class RegisterPage {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  protected readonly username = signal('');
  protected readonly password = signal('');
  protected readonly confirmPassword = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly fieldErrors = signal<Record<string, string>>({});

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    const username = this.username().trim();
    const password = this.password();
    if (password !== this.confirmPassword()) {
      this.fieldErrors.set({ confirmPassword: 'Passwords do not match.' });
      return;
    }

    this.busy.set(true);
    this.error.set(null);
    this.fieldErrors.set({});
    this.auth.register(username, password).subscribe({
      next: () => {
        this.busy.set(false);
        void this.router.navigate(['/profile']);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.password.set('');
        this.confirmPassword.set('');
        this.fieldErrors.set(apiFieldErrors(err));
        this.error.set(apiMessage(err, 'Could not create the account.'));
      },
    });
  }
}
