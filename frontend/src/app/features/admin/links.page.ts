import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ApiService, apiFieldErrors, apiMessage } from '../../core/api.service';
import { AdminLink, LinkPayload } from '../../core/models';

/** Empty draft used when the "Add link" form is opened. */
const EMPTY_DRAFT: LinkPayload = { title: '', url: '', icon: '', enabled: true };

/**
 * Link management: create, edit, enable/disable, delete and reorder.
 *
 * Reordering supports pointer drag-and-drop and, for touch and keyboard users,
 * explicit move up/down buttons. Both paths funnel into the same PUT that
 * sends the complete id order to the API.
 */
@Component({
  selector: 'app-admin-links',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, NgTemplateOutlet],
  templateUrl: './links.page.html',
  styleUrl: './links.page.css',
})
export class LinksPage implements OnInit {
  private readonly api = inject(ApiService);

  protected readonly links = signal<AdminLink[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly notice = signal<string | null>(null);

  /** null = closed, 0 = creating a new link, >0 = editing that link id. */
  protected readonly editing = signal<number | null>(null);
  protected readonly draft = signal<LinkPayload>({ ...EMPTY_DRAFT });
  protected readonly fieldErrors = signal<Record<string, string>>({});
  protected readonly saving = signal(false);
  protected readonly pendingDelete = signal<number | null>(null);
  protected readonly busyId = signal<number | null>(null);

  /** Index of the row currently being dragged, if any. */
  protected readonly dragIndex = signal<number | null>(null);
  protected readonly dropIndex = signal<number | null>(null);

  protected readonly enabledCount = computed(() => this.links().filter((l) => l.enabled).length);

  ngOnInit(): void {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.error.set(null);

    this.api.links().subscribe({
      next: (res) => {
        this.links.set(res.links);
        this.loading.set(false);
      },
      error: (err: unknown) => {
        this.error.set(apiMessage(err, 'Could not load links.'));
        this.loading.set(false);
      },
    });
  }

  // --- editor ---------------------------------------------------------------

  protected startCreate(): void {
    this.editing.set(0);
    this.draft.set({ ...EMPTY_DRAFT });
    this.fieldErrors.set({});
    this.pendingDelete.set(null);
  }

  protected startEdit(link: AdminLink): void {
    this.editing.set(link.id);
    this.draft.set({
      title: link.title,
      url: link.url,
      icon: link.icon,
      enabled: link.enabled,
    });
    this.fieldErrors.set({});
    this.pendingDelete.set(null);
  }

  protected cancelEdit(): void {
    this.editing.set(null);
    this.fieldErrors.set({});
  }

  protected patchDraft(patch: Partial<LinkPayload>): void {
    this.draft.update((current) => ({ ...current, ...patch }));
  }

  protected save(): void {
    const id = this.editing();
    if (id === null || this.saving()) {
      return;
    }

    this.saving.set(true);
    this.fieldErrors.set({});
    const payload = this.draft();

    const request = id === 0 ? this.api.createLink(payload) : this.api.updateLink(id, payload);
    request.subscribe({
      next: (link) => {
        this.saving.set(false);
        this.editing.set(null);
        this.links.update((links) =>
          id === 0 ? [...links, link] : links.map((l) => (l.id === link.id ? link : l)),
        );
        this.flash(id === 0 ? 'Link added.' : 'Link saved.');
      },
      error: (err: unknown) => {
        this.saving.set(false);
        this.fieldErrors.set(apiFieldErrors(err));
        this.error.set(apiMessage(err, 'Could not save the link.'));
      },
    });
  }

  // --- row actions ----------------------------------------------------------

  protected toggleEnabled(link: AdminLink): void {
    this.busyId.set(link.id);
    this.api
      .updateLink(link.id, {
        title: link.title,
        url: link.url,
        icon: link.icon,
        enabled: !link.enabled,
      })
      .subscribe({
        next: (updated) => {
          this.busyId.set(null);
          this.links.update((links) => links.map((l) => (l.id === updated.id ? updated : l)));
        },
        error: (err: unknown) => {
          this.busyId.set(null);
          this.error.set(apiMessage(err, 'Could not update the link.'));
        },
      });
  }

  protected confirmDelete(link: AdminLink): void {
    this.pendingDelete.set(link.id);
  }

  protected cancelDelete(): void {
    this.pendingDelete.set(null);
  }

  protected remove(link: AdminLink): void {
    this.busyId.set(link.id);
    this.api.deleteLink(link.id).subscribe({
      next: () => {
        this.busyId.set(null);
        this.pendingDelete.set(null);
        this.links.update((links) => links.filter((l) => l.id !== link.id));
        this.flash('Link deleted.');
      },
      error: (err: unknown) => {
        this.busyId.set(null);
        this.error.set(apiMessage(err, 'Could not delete the link.'));
      },
    });
  }

  // --- ordering -------------------------------------------------------------

  /** Moves a row by delta positions (used by the up/down buttons). */
  protected move(index: number, delta: number): void {
    const target = index + delta;
    const links = this.links();
    if (target < 0 || target >= links.length) {
      return;
    }
    this.applyOrder(reorder(links, index, target));
  }

  protected onDragStart(index: number, event: DragEvent): void {
    this.dragIndex.set(index);
    event.dataTransfer?.setData('text/plain', String(index));
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  protected onDragOver(index: number, event: DragEvent): void {
    if (this.dragIndex() === null) {
      return;
    }
    event.preventDefault(); // required for the drop event to fire
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
    this.dropIndex.set(index);
  }

  protected onDrop(index: number, event: DragEvent): void {
    event.preventDefault();
    const from = this.dragIndex();
    this.onDragEnd();
    if (from === null || from === index) {
      return;
    }
    this.applyOrder(reorder(this.links(), from, index));
  }

  protected onDragEnd(): void {
    this.dragIndex.set(null);
    this.dropIndex.set(null);
  }

  /** Optimistically applies a new order, then persists it. */
  private applyOrder(next: AdminLink[]): void {
    const previous = this.links();
    this.links.set(next.map((link, position) => ({ ...link, position })));

    this.api.reorderLinks(next.map((link) => link.id)).subscribe({
      next: (res) => this.links.set(res.links),
      error: (err: unknown) => {
        this.links.set(previous); // roll back to the server's last known order
        this.error.set(apiMessage(err, 'Could not save the new order.'));
      },
    });
  }

  // --- misc -----------------------------------------------------------------

  protected dismissError(): void {
    this.error.set(null);
  }

  private flash(message: string): void {
    this.error.set(null);
    this.notice.set(message);
    setTimeout(() => this.notice.set(null), 2500);
  }
}

/** Returns a copy of items with the element at `from` moved to `to`. */
function reorder<T>(items: readonly T[], from: number, to: number): T[] {
  const next = [...items];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}
