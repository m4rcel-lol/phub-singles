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
      { path: '', pathMatch: 'full', redirectTo: 'links' },
      {
        path: 'links',
        title: 'Links · Admin',
        loadComponent: () => import('./links.page').then((m) => m.LinksPage),
      },
      {
        path: 'profile',
        title: 'Profile · Admin',
        loadComponent: () => import('./profile.page').then((m) => m.ProfilePage),
      },
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
      {
        path: 'account',
        title: 'Account · Admin',
        loadComponent: () => import('./account.page').then((m) => m.AccountPage),
      },
      { path: '**', redirectTo: 'links' },
    ],
  },
];
