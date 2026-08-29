import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Observable } from 'rxjs';

import { ApiService, apiFieldErrors, apiMessage } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { AdminUser, Role } from '../../core/models';
import { BadgeList } from '../../shared/badge-list';

interface Draft {
  username: string;
  password: string;
  role: Role;
}

const EMPTY_DRAFT: Draft = { username: '', password: '', role: 'member' };

/**
 * The admin panel's people screen.
 *
 * What an account may do here is decided by the server and returned per row
 * (`canManage`, `canChangeRole`); the UI only mirrors it, so an administrator
 * never sees a button that would come back 403. Roles are owner-only, and
 * ownership itself is not transferable from any panel.
 */
@Component({
  selector: 'app-admin-users',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, BadgeList],
  templateUrl: './users.page.html',
  styleUrl: './users.page.css',
})
export class UsersPage implements OnInit {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);

  protected readonly users = signal<AdminUser[]>([]);
  protected readonly views = signal(0);
  protected readonly threshold = signal(10000);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);
  protected readonly busy = signal<string | null>(null);

  protected readonly isOwner = computed(() => this.auth.role() === 'owner');

  // Create form.
  protected readonly creating = signal(false);
  protected readonly draft = signal<Draft>({ ...EMPTY_DRAFT });
  protected readonly fieldErrors = signal<Record<string, string>>({});

  // Per-row interactions.
  protected readonly pendingDelete = signal<string | null>(null);
  protected readonly resetting = signal<string | null>(null);
  protected readonly newPassword = signal('');

  /** Progress towards the automatic Verified badge, as a percentage. */
  protected readonly progress = computed(() =>
    Math.min(100, Math.round((this.views() / Math.max(1, this.threshold())) * 100)),
  );

  ngOnInit(): void {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.error.set(null);

    this.api.users().subscribe({
      next: (payload) => {
        this.users.set(payload.users);
        this.views.set(payload.views);
        this.threshold.set(payload.threshold);
        this.loading.set(false);
      },
      error: (err: unknown) => {
        this.error.set(apiMessage(err, 'Could not load accounts.'));
        this.loading.set(false);
      },
    });
  }

  // --- create ---------------------------------------------------------------

  protected startCreate(): void {
    this.creating.set(true);
    this.draft.set({ ...EMPTY_DRAFT });
    this.fieldErrors.set({});
  }

  protected cancelCreate(): void {
    this.creating.set(false);
    this.fieldErrors.set({});
  }

  protected patchDraft(patch: Partial<Draft>): void {
    this.draft.update((current) => ({ ...current, ...patch }));
  }

  protected create(): void {
    if (this.busy()) {
      return;
    }
    const draft = this.draft();

    this.busy.set('create');
    this.fieldErrors.set({});

    this.api.createUser(draft.username, draft.password, draft.role).subscribe({
      next: (user) => {
        this.busy.set(null);
        this.creating.set(false);
        this.flash(`${user.username} added as ${user.role}.`);
        this.load();
      },
      error: (err: unknown) => {
        this.busy.set(null);
        this.fieldErrors.set(apiFieldErrors(err));
        this.error.set(apiMessage(err, 'Could not create the account.'));
      },
    });
  }

  // --- row actions ----------------------------------------------------------

  protected toggleVerified(user: AdminUser): void {
    this.run(user.username, this.api.setVerified(user.username, !user.verifiedAt), () =>
      user.verifiedAt
        ? `Verification removed from ${user.username}.`
        : `${user.username} is verified.`,
    );
  }

  protected setRole(user: AdminUser, role: Role): void {
    this.run(user.username, this.api.setRole(user.username, role), () =>
      role === 'admin'
        ? `${user.username} is now an administrator.`
        : `${user.username} is no longer an administrator.`,
    );
  }

  protected confirmDelete(user: AdminUser): void {
    this.pendingDelete.set(user.username);
    this.resetting.set(null);
  }

  protected remove(user: AdminUser): void {
    this.pendingDelete.set(null);
    this.run(user.username, this.api.deleteUser(user.username), () => `${user.username} deleted.`);
  }

  protected startReset(user: AdminUser): void {
    this.resetting.set(user.username);
    this.newPassword.set('');
    this.pendingDelete.set(null);
  }

  protected submitReset(user: AdminUser): void {
    if (this.newPassword().length < 8) {
      this.error.set('The new password must be at least 8 characters.');
      return;
    }
    this.resetting.set(null);
    this.run(
      user.username,
      this.api.resetPassword(user.username, this.newPassword()),
      () => `Password reset for ${user.username}; their sessions were revoked.`,
    );
  }

  protected cancelRow(): void {
    this.pendingDelete.set(null);
    this.resetting.set(null);
  }

  /** Shared plumbing for the one-shot row actions. */
  private run(username: string, request: Observable<unknown>, message: () => string): void {
    if (this.busy()) {
      return;
    }
    this.busy.set(username);
    this.error.set(null);

    request.subscribe({
      next: () => {
        this.busy.set(null);
        this.flash(message());
        this.load();
      },
      error: (err: unknown) => {
        this.busy.set(null);
        this.error.set(apiMessage(err, 'That did not work.'));
      },
    });
  }

  private flash(message: string): void {
    this.notice.set(message);
    setTimeout(() => this.notice.set(null), 3000);
  }
}
