import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import {
  AdminLink,
  AdminUser,
  ApiErrorBody,
  LinkPayload,
  PagePayload,
  Profile,
  SessionInfo,
  Stats,
  Role,
  SettingsPayload,
  SiteSettings,
  UsersPayload,
} from './models';

/** Usernames go into the path, so they are escaped exactly once, here. */
function user(username: string): string {
  return encodeURIComponent(username);
}

/** Everything is same-origin: Caddy proxies the app and the API to one host. */
const API = '/api';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);

  // --- public ---------------------------------------------------------------

  /**
   * Loads the public page. Passing a handle makes the server answer 404 when
   * it does not match the published profile, so /<handle> resolves correctly.
   */
  page(handle?: string): Observable<PagePayload> {
    const options = handle ? { params: { handle } } : {};
    return this.http.get<PagePayload>(`${API}/page`, options);
  }

  /**
   * Analytics beacons are fire-and-forget: a failed counter must never block
   * navigation, so they bypass HttpClient and use keepalive fetch instead.
   */
  registerView(): void {
    this.beacon(`${API}/views`);
  }

  registerClick(id: number): void {
    this.beacon(`${API}/links/${id}/click`);
  }

  private beacon(url: string): void {
    void fetch(url, { method: 'POST', keepalive: true, credentials: 'same-origin' }).catch(
      () => undefined,
    );
  }

  // --- session --------------------------------------------------------------

  login(username: string, password: string): Observable<SessionInfo> {
    return this.http.post<SessionInfo>(`${API}/admin/login`, { username, password });
  }

  logout(): Observable<void> {
    return this.http.post<void>(`${API}/admin/logout`, {});
  }

  /** Public: reports whether the caller has a session, without 401ing. */
  session(): Observable<SessionInfo> {
    return this.http.get<SessionInfo>(`${API}/session`);
  }

  changePassword(currentPassword: string, newPassword: string): Observable<unknown> {
    return this.http.post(`${API}/admin/password`, { currentPassword, newPassword });
  }

  // --- profile --------------------------------------------------------------

  adminProfile(): Observable<Profile> {
    return this.http.get<Profile>(`${API}/admin/profile`);
  }

  updateProfile(
    profile: Pick<Profile, 'username' | 'displayName' | 'tagline' | 'bio'>,
  ): Observable<Profile> {
    return this.http.put<Profile>(`${API}/admin/profile`, profile);
  }

  uploadAvatar(file: File): Observable<Profile> {
    const form = new FormData();
    form.append('avatar', file);
    return this.http.post<Profile>(`${API}/admin/profile/avatar`, form);
  }

  deleteAvatar(): Observable<Profile> {
    return this.http.delete<Profile>(`${API}/admin/profile/avatar`);
  }

  // --- links ----------------------------------------------------------------

  links(): Observable<{ links: AdminLink[] }> {
    return this.http.get<{ links: AdminLink[] }>(`${API}/admin/links`);
  }

  createLink(payload: LinkPayload): Observable<AdminLink> {
    return this.http.post<AdminLink>(`${API}/admin/links`, payload);
  }

  updateLink(id: number, payload: LinkPayload): Observable<AdminLink> {
    return this.http.put<AdminLink>(`${API}/admin/links/${id}`, payload);
  }

  deleteLink(id: number): Observable<void> {
    return this.http.delete<void>(`${API}/admin/links/${id}`);
  }

  reorderLinks(ids: number[]): Observable<{ links: AdminLink[] }> {
    return this.http.put<{ links: AdminLink[] }>(`${API}/admin/links/order`, { ids });
  }

  // --- accounts and badges --------------------------------------------------

  users(): Observable<UsersPayload> {
    return this.http.get<UsersPayload>(`${API}/admin/users`);
  }

  setVerified(username: string, verified: boolean): Observable<AdminUser> {
    return this.http.put<AdminUser>(`${API}/admin/users/${user(username)}/verified`, { verified });
  }

  createUser(username: string, password: string, role: Role): Observable<AdminUser> {
    return this.http.post<AdminUser>(`${API}/admin/users`, { username, password, role });
  }

  deleteUser(username: string): Observable<void> {
    return this.http.delete<void>(`${API}/admin/users/${user(username)}`);
  }

  /** Owner only. */
  setRole(username: string, role: Role): Observable<AdminUser> {
    return this.http.put<AdminUser>(`${API}/admin/users/${user(username)}/role`, { role });
  }

  resetPassword(username: string, password: string): Observable<unknown> {
    return this.http.post(`${API}/admin/users/${user(username)}/password`, { password });
  }

  // --- site settings (owner only) -------------------------------------------

  settings(): Observable<SettingsPayload> {
    return this.http.get<SettingsPayload>(`${API}/admin/settings`);
  }

  updateSettings(settings: SiteSettings): Observable<{ settings: SiteSettings }> {
    return this.http.put<{ settings: SiteSettings }>(`${API}/admin/settings`, settings);
  }

  // --- stats ----------------------------------------------------------------

  stats(days = 14): Observable<Stats> {
    return this.http.get<Stats>(`${API}/admin/stats`, { params: { days } });
  }
}

/** Extracts the human-readable message from an API error response. */
export function apiMessage(error: unknown, fallback = 'Something went wrong.'): string {
  const body = errorBody(error);
  return body?.message ?? fallback;
}

/** Returns the machine-readable error code, e.g. "maintenance". */
export function apiErrorCode(error: unknown): string | null {
  return errorBody(error)?.error ?? null;
}

/** Extracts per-field validation messages, if the server sent any. */
export function apiFieldErrors(error: unknown): Record<string, string> {
  return errorBody(error)?.fields ?? {};
}

function errorBody(error: unknown): ApiErrorBody | null {
  if (error instanceof HttpErrorResponse && error.error && typeof error.error === 'object') {
    return error.error as ApiErrorBody;
  }
  return null;
}
