import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, map, of, tap } from 'rxjs';

import { ApiService } from './api.service';
import { SessionInfo } from './models';

const ANONYMOUS: SessionInfo = { authenticated: false, isAdmin: false };

/**
 * Session state for the admin panel. The session itself lives in an HttpOnly
 * cookie, so the client can only observe it by asking the server; the signal
 * below caches that answer for the lifetime of the page.
 */
@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly api = inject(ApiService);

  /** null until the first answer arrives from the server. */
  private readonly session = signal<SessionInfo | null>(null);

  readonly resolved = computed(() => this.session() !== null);
  readonly isAuthenticated = computed(() => this.session()?.authenticated ?? false);
  readonly isAdmin = computed(() => this.session()?.isAdmin ?? false);
  readonly username = computed(() => this.session()?.username ?? null);
  readonly role = computed(() => this.session()?.role ?? null);

  /** Resolves the current session, hitting the API at most once per page load. */
  ensureSession(): Observable<SessionInfo> {
    const cached = this.session();
    if (cached) {
      return of(cached);
    }
    return this.refresh();
  }

  /** Re-reads the session, bypassing the cache. */
  refresh(): Observable<SessionInfo> {
    return this.api.session().pipe(
      tap((session) => this.session.set(session)),
      catchError(() => {
        // A network failure must not look like "signed in".
        this.session.set(ANONYMOUS);
        return of(ANONYMOUS);
      }),
    );
  }

  login(username: string, password: string): Observable<void> {
    return this.api.login(username, password).pipe(
      tap((session) => this.session.set(session)),
      map(() => undefined),
    );
  }

  logout(): Observable<void> {
    return this.api.logout().pipe(
      tap(() => this.clear()),
      catchError(() => {
        this.clear();
        return of(undefined);
      }),
      map(() => undefined),
    );
  }

  /** Drops the cached session, e.g. after a 401 or a password change. */
  clear(): void {
    this.session.set(ANONYMOUS);
  }
}
