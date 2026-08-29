import { Routes } from '@angular/router';

import { authGuard, guestGuard, ownerGuard } from '../../core/auth.guard';

/** Lazily loaded admin area. Each screen is its own chunk. */
export const adminRoutes: Routes = [
  {
    path: 'login',
    canActivate: [guestGuard],
    title: 'Sign in · pornhub.singles',
    loadComponent: () => import('./login.page').then((m) => m.LoginPage),
  },
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () => import('./shell.page').then((m) => m.AdminShell),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'stats' },
      {
        path: 'users',
        title: 'People · Admin',
        loadComponent: () => import('./users.page').then((m) => m.UsersPage),
      },
      {
        path: 'site',
        canActivate: [ownerGuard],
        title: 'Site settings · Owner',
        loadComponent: () => import('./site.page').then((m) => m.SitePage),
      },
      {
        path: 'stats',
        title: 'Stats · Admin',
        loadComponent: () => import('./stats.page').then((m) => m.StatsPage),
      },
      { path: '**', redirectTo: 'stats' },
    ],
  },
];
