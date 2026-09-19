import { Routes } from '@angular/router';

import { authGuard, guestGuard, studentGuard } from './auth/auth.guard';

export const routes: Routes = [
  {
    path: 'login',
    canActivate: [guestGuard],
    loadComponent: () => import('./pages/login/login.component').then((module) => module.LoginComponent)
  },
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () => import('./layout/app-shell.component').then((module) => module.AppShellComponent),
    children: [
      {
        path: 'dashboard',
        loadComponent: () =>
          import('./pages/dashboard/dashboard.component').then((module) => module.DashboardComponent)
      },
      {
        path: 'student/application',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-application/student-application.component').then(
            (module) => module.StudentApplicationComponent
          )
      }
    ]
  },
  { path: '', pathMatch: 'full', redirectTo: 'login' },
  { path: '**', redirectTo: 'login' }
];
