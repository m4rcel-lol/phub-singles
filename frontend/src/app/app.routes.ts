import { Routes } from '@angular/router';

import { HomePage } from './features/home/home.page';
import { LandingPage } from './features/landing/landing.page';

const SITE = 'pornhub.singles';

/**
 * Route order matters: the fixed pages are declared before the catch-all
 * `:handle`, which resolves the public profile. The server enforces the same
 * reservations when a handle is saved (see reservedHandles in the API).
 */
export const routes: Routes = [
  {
    path: '',
    pathMatch: 'full',
    component: LandingPage,
    title: SITE,
  },
  {
    path: 'admin',
    loadChildren: () => import('./features/admin/admin.routes').then((m) => m.adminRoutes),
  },
  {
    path: 'notice',
    title: `Notice · ${SITE}`,
    loadComponent: () => import('./features/legal/notice.page').then((m) => m.NoticePage),
  },
  {
    path: 'privacy',
    title: `Privacy policy · ${SITE}`,
    loadComponent: () => import('./features/legal/privacy.page').then((m) => m.PrivacyPage),
  },
  {
    path: 'terms',
    title: `Terms of service · ${SITE}`,
    loadComponent: () => import('./features/legal/terms.page').then((m) => m.TermsPage),
  },
  {
    path: 'not-found',
    title: `Not found · ${SITE}`,
    loadComponent: () => import('./features/legal/not-found.page').then((m) => m.NotFoundPage),
  },
  {
    // The public bio page: /<handle>. Kept in the initial bundle alongside the
    // landing page — these two are the entry points for real visitors, and a
    // lazy chunk would only add a round trip to the page that matters most.
    path: ':handle',
    component: HomePage,
  },
  { path: '**', redirectTo: 'not-found' },
];
