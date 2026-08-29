import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';

import { ApiService, apiFieldErrors, apiMessage } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';

@Component({
  selector: 'app-admin-account',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  templateUrl: './account.page.html',
  styleUrl: './account.page.css',
})
export class AccountPage {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  protected readonly username = this.auth.username;

  protected readonly current = signal('');
  protected readonly next = signal('');
  protected readonly confirm = signal('');

  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);
  protected readonly fieldErrors = signal<Record<string, string>>({});

  protected submit(): void {
    if (this.saving()) {
      return;
    }
    this.error.set(null);
    this.fieldErrors.set({});

    if (this.next().length < 8) {
      this.fieldErrors.set({ newPassword: 'Must be at least 8 characters.' });
      return;
    }
    if (this.next() !== this.confirm()) {
      this.fieldErrors.set({ confirm: 'The two passwords do not match.' });
      return;
    }

    this.saving.set(true);
    this.api.changePassword(this.current(), this.next()).subscribe({
      next: () => {
        this.saving.set(false);
        // The server revoked every session, this one included.
        this.notice.set('Password changed. Signing you in again…');
        this.auth.clear();
        setTimeout(() => void this.router.navigate(['/admin/login']), 1200);
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.fieldErrors.set(apiFieldErrors(err));
        this.error.set(apiMessage(err, 'Could not change the password.'));
      },
    });
  }
}
