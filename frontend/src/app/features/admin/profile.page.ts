import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiService, apiFieldErrors, apiMessage } from '../../core/api.service';
import { Profile } from '../../core/models';

/** Limits mirrored from the server so the UI can count down as you type. */
const LIMITS = { username: 32, displayName: 60, tagline: 120, bio: 400 } as const;

@Component({
  selector: 'app-admin-profile',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  templateUrl: './profile.page.html',
  styleUrl: './profile.page.css',
})
export class ProfilePage implements OnInit {
  private readonly api = inject(ApiService);

  protected readonly limits = LIMITS;

  protected readonly profile = signal<Profile | null>(null);
  protected readonly username = signal('');
  protected readonly displayName = signal('');
  protected readonly tagline = signal('');
  protected readonly bio = signal('');

  protected readonly loading = signal(true);
  protected readonly saving = signal(false);
  protected readonly uploading = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);
  protected readonly fieldErrors = signal<Record<string, string>>({});

  /** Where the public page will live once saved. */
  protected readonly publicPath = computed(() => `/${this.username()}`);

  protected readonly initial = computed(() => {
    const name = this.displayName().trim();
    return name ? [...name][0].toUpperCase() : '•';
  });

  ngOnInit(): void {
    this.api.adminProfile().subscribe({
      next: (profile) => {
        this.apply(profile);
        this.loading.set(false);
      },
      error: (err: unknown) => {
        this.error.set(apiMessage(err, 'Could not load the profile.'));
        this.loading.set(false);
      },
    });
  }

  protected save(): void {
    if (this.saving()) {
      return;
    }
    this.saving.set(true);
    this.fieldErrors.set({});
    this.error.set(null);

    this.api
      .updateProfile({
        username: this.username(),
        displayName: this.displayName(),
        tagline: this.tagline(),
        bio: this.bio(),
      })
      .subscribe({
        next: (profile) => {
          this.saving.set(false);
          this.apply(profile);
          this.flash('Profile saved.');
        },
        error: (err: unknown) => {
          this.saving.set(false);
          this.fieldErrors.set(apiFieldErrors(err));
          this.error.set(apiMessage(err, 'Could not save the profile.'));
        },
      });
  }

  protected onAvatarSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = ''; // allow re-selecting the same file after an error
    if (!file) {
      return;
    }

    this.uploading.set(true);
    this.error.set(null);

    this.api.uploadAvatar(file).subscribe({
      next: (profile) => {
        this.uploading.set(false);
        this.apply(profile);
        this.flash('Avatar updated.');
      },
      error: (err: unknown) => {
        this.uploading.set(false);
        this.error.set(apiMessage(err, 'Could not upload the image.'));
      },
    });
  }

  protected removeAvatar(): void {
    this.uploading.set(true);
    this.api.deleteAvatar().subscribe({
      next: (profile) => {
        this.uploading.set(false);
        this.apply(profile);
        this.flash('Avatar removed.');
      },
      error: (err: unknown) => {
        this.uploading.set(false);
        this.error.set(apiMessage(err, 'Could not remove the image.'));
      },
    });
  }

  private apply(profile: Profile): void {
    this.profile.set(profile);
    this.username.set(profile.username);
    this.displayName.set(profile.displayName);
    this.tagline.set(profile.tagline);
    this.bio.set(profile.bio);
  }

  private flash(message: string): void {
    this.notice.set(message);
    setTimeout(() => this.notice.set(null), 2500);
  }
}
