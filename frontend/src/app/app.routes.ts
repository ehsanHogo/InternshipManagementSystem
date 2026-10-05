import { Routes } from '@angular/router';

import {
  authGuard,
  currentAccountGuard,
  universityIdentityGuard,
  adminGuard,
  companyAccessGuard,
  companySupervisorGuard,
  guestGuard,
  professorGuard,
  studentGuard,
  universitySupervisorGuard
} from './auth/auth.guard';

export const routes: Routes = [
 { path: 'university-supervisor-register', canActivate: [guestGuard], loadComponent: () => import('./pages/university-register/university-register.component').then(m => m.UniversityRegisterComponent) },
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
    canActivateChild: [currentAccountGuard, companyAccessGuard],
    loadComponent: () => import('./layout/app-shell.component').then((module) => module.AppShellComponent),
    children: [
      { path: 'university-supervisor/verification', canActivate: [universityIdentityGuard], loadComponent: () => import('./pages/university-verification/university-verification.component').then(m => m.UniversityVerificationComponent) },
      { path: 'admin/user-verifications', canActivate: [adminGuard], data: { verificationOnly: true }, loadComponent: () => import('./pages/admin-users/admin-users.component').then(m => m.AdminUsersComponent) },
      { path: 'admin/users', canActivate: [adminGuard], data: { verificationOnly: false }, loadComponent: () => import('./pages/admin-users/admin-users.component').then(m => m.AdminUsersComponent) },
      {
        path: 'profile',
        loadComponent: () => import('./pages/profile/profile.component').then(module => module.ProfileComponent)
      },
      {
        path: '',
        pathMatch: 'full',
        redirectTo: 'dashboard'
      },
      {
        path: 'dashboard',
        loadComponent: () =>
          import('./pages/dashboard/dashboard.component').then((module) => module.DashboardComponent)
      },
      {
        path: 'admin/company-registrations',
        canActivate: [adminGuard],
        loadComponent: () => import('./pages/admin-company-registrations/admin-company-registrations.component').then(module => module.AdminCompanyRegistrationsComponent)
      },
      {
        path: 'student/opportunities',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-opportunities/student-opportunities.component').then(
            (module) => module.StudentOpportunitiesComponent
          )
      },
      {
        path: 'student/opportunities/:id',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-opportunity-detail/student-opportunity-detail.component').then(
            (module) => module.StudentOpportunityDetailComponent
          )
      },
      {
        path: 'student/opportunity-applications',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-opportunity-applications/student-opportunity-applications.component').then(
            (module) => module.StudentOpportunityApplicationsComponent
          )
      },
      {
        path: 'student/opportunity-applications/:id',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-opportunity-application-detail/student-opportunity-application-detail.component').then(
            (module) => module.StudentOpportunityApplicationDetailComponent
          )
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
        path: 'student/approved-companies',
        canActivate: [studentGuard],
        loadComponent: () =>
          import('./pages/student-approved-companies/student-approved-companies.component').then(
            (module) => module.StudentApprovedCompaniesComponent
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
        path: 'university/internship-terms',
        canActivate: [universitySupervisorGuard],
        loadComponent: () => import('./pages/university-internship-terms/university-internship-terms.component')
          .then(module => module.UniversityInternshipTermsComponent)
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
        path: 'company/opportunities',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-opportunities/company-opportunities.component').then(
            (module) => module.CompanyOpportunitiesComponent
          )
      },
      {
        path: 'company/opportunities/:id/applications',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-opportunity-applications/company-opportunity-applications.component').then(
            (module) => module.CompanyOpportunityApplicationsComponent
          )
      },
      {
        path: 'company/opportunity-applications/:id',
        canActivate: [companySupervisorGuard],
        loadComponent: () =>
          import('./pages/company-opportunity-application-detail/company-opportunity-application-detail.component').then(
            (module) => module.CompanyOpportunityApplicationDetailComponent
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
      },
      {
        path: '**',
        redirectTo: 'dashboard'
      }
    ]
  },
  { path: '**', redirectTo: 'login' }
];
