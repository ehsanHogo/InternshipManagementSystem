import { Routes } from '@angular/router';

import {
  authGuard,
  companySupervisorGuard,
  guestGuard,
  studentGuard,
  universitySupervisorGuard
} from './auth/auth.guard';

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
      },
      {
        path: 'university/applications',
        canActivate: [universitySupervisorGuard],
        loadComponent: () =>
          import('./pages/university-applications/university-applications.component').then(
            (module) => module.UniversityApplicationsComponent
          )
      },
      {
        path: 'university/applications/:id',
        canActivate: [universitySupervisorGuard],
        loadComponent: () =>
          import('./pages/university-case/university-case.component').then(
            (module) => module.UniversityCaseComponent
          )
      },
      {
        path: 'company/internships',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-internships/company-internships.component').then(
            (module) => module.CompanyInternshipsComponent
          )
      },
      {
        path: 'company/internships/:id',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-case/company-case.component').then(
            (module) => module.CompanyCaseComponent
          )
      }
    ]
  },
  { path: '', pathMatch: 'full', redirectTo: 'login' },
  { path: '**', redirectTo: 'login' }
];
