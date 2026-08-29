import { Routes } from '@angular/router';

import { signedInGuard } from '../../core/auth.guard';

/** The account dashboard is available to every signed-in user. */
export const dashboardRoutes: Routes = [
  {
    path: '',
    canActivate: [signedInGuard],
    loadComponent: () => import('./shell.page').then((m) => m.DashboardShell),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'profile' },
      {
        path: 'profile',
        title: 'Profile · Dashboard',
        loadComponent: () => import('../admin/profile.page').then((m) => m.ProfilePage),
      },
      {
        path: 'links',
        title: 'Links · Dashboard',
        loadComponent: () => import('../admin/links.page').then((m) => m.LinksPage),
      },
      {
        path: 'account',
        title: 'Account · Dashboard',
        loadComponent: () => import('../admin/account.page').then((m) => m.AccountPage),
      },
      { path: '**', redirectTo: 'profile' },
    ],
  },
];
