import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { map } from 'rxjs';

import { AuthService } from './auth.service';

/**
 * Blocks the admin area for anonymous visitors and for accounts whose
 * administrative access has been revoked. The attempted URL is carried along so
 * signing in returns to where the user was headed.
 */
export const authGuard: CanActivateFn = (_route, state) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth.ensureSession().pipe(
    map(
      (session) =>
        session.isAdmin ||
        router.createUrlTree(['/admin/login'], {
          queryParams: state.url === '/admin' ? {} : { next: state.url },
        }),
    ),
  );
};

/** The owner panel: site settings and role changes. */
export const ownerGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth
    .ensureSession()
    .pipe(map((session) => session.role === 'owner' || router.createUrlTree(['/admin/links'])));
};

/** Keeps a signed-in admin from landing back on the login form. */
export const guestGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return auth.ensureSession().pipe(map((session) => !session.isAdmin || router.createUrlTree(['/admin'])));
};
