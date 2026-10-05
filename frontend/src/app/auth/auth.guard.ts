import { inject } from '@angular/core';
import { CanActivateFn, CanActivateChildFn, Router } from '@angular/router';

import { catchError, map, of } from 'rxjs';

import { AuthService } from './auth.service';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.isAuthenticated() ? true : router.createUrlTree(['/login']);
};

export const guestGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.isAuthenticated() ? router.createUrlTree(['/dashboard']) : true;
};

export const studentGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.getCurrentUser()?.role === 'STUDENT' ? true : router.createUrlTree(['/dashboard']);
};

export const universitySupervisorGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.getCurrentUser()?.role === 'UNIVERSITY_SUPERVISOR' ? true : router.createUrlTree(['/dashboard']);
};

export const companySupervisorGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.getCurrentUser()?.role === 'COMPANY_SUPERVISOR' ? true : router.createUrlTree(['/dashboard']);
};

export const professorGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.getCurrentUser()?.role === 'PROFESSOR' ? true : router.createUrlTree(['/dashboard']);
};

// Refresh the permission on every authenticated child navigation so an Admin
// decision takes effect for an already logged-in company without a new JWT.
export const companyAccessGuard: CanActivateChildFn = (_route, state) => {
 const auth = inject(AuthService);
 const router = inject(Router);
 if (auth.getCurrentUser()?.role !== 'COMPANY_SUPERVISOR') return true;
 return auth.refreshUser().pipe(
  map(user => user.companyRegistrationStatus === 'APPROVED' || state.url.split('?')[0] === '/company/profile'
    ? true : router.createUrlTree(['/company/profile'])),
  catchError(() => of(state.url.split('?')[0] === '/company/profile' ? true : router.createUrlTree(['/company/profile'])))
 );
};
export const adminGuard: CanActivateFn = () => {
 const auth = inject(AuthService);
 const router = inject(Router);
 return auth.getCurrentUser()?.role === 'ADMIN' ? true : router.createUrlTree(['/dashboard']);
};
