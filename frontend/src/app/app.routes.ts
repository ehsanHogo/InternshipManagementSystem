import { Routes } from '@angular/router';

import {
  authGuard,
  companySupervisorGuard,
  guestGuard,
  professorGuard,
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
    path: 'company-register',
    canActivate: [guestGuard],
    loadComponent: () =>
      import('./pages/company-register/company-register.component').then((module) => module.CompanyRegisterComponent)
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
        path: 'student/weekly-reports',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-weekly-reports/student-weekly-reports.component').then(
            (module) => module.StudentWeeklyReportsComponent
          )
      },
      {
        path: 'student/final-report',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-final-report/student-final-report.component').then(
            (module) => module.StudentFinalReportComponent
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
        path: 'university/students',
        canActivate: [universitySupervisorGuard],
        data: { kind: 'students' },
        loadComponent: () =>
          import('./pages/university-users/university-users.component').then(
            (module) => module.UniversityUsersComponent
          )
      },
      {
        path: 'university/professors',
        canActivate: [universitySupervisorGuard],
        data: { kind: 'professors' },
        loadComponent: () =>
          import('./pages/university-users/university-users.component').then(
            (module) => module.UniversityUsersComponent
          )
      },
      {
        path: 'university/companies',
        canActivate: [universitySupervisorGuard],
        loadComponent: () =>
          import('./pages/university-companies/university-companies.component').then(
            (module) => module.UniversityCompaniesComponent
          )
      },
      {
        path: 'university/company-supervisors',
        canActivate: [universitySupervisorGuard],
        loadComponent: () =>
          import('./pages/university-company-supervisors/university-company-supervisors.component').then(
            (module) => module.UniversityCompanySupervisorsComponent
          )
      },
      {
        path: 'university/professor-assignments',
        canActivate: [universitySupervisorGuard],
        loadComponent: () =>
          import('./pages/university-professor-assignments/university-professor-assignments.component').then(
            (module) => module.UniversityProfessorAssignmentsComponent
          )
      },
      {
        path: 'professor/internships',
        canActivate: [professorGuard],
        loadComponent: () =>
          import('./pages/professor-internships/professor-internships.component').then(
            (module) => module.ProfessorInternshipsComponent
          )
      },
      {
        path: 'professor/internships/:id',
        canActivate: [professorGuard],
        loadComponent: () =>
          import('./pages/professor-case/professor-case.component').then(
            (module) => module.ProfessorCaseComponent
          )
      },
      {
        path: 'company/profile',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-profile/company-profile.component').then(
            (module) => module.CompanyProfileComponent
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
