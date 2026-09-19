import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

import { getStoredToken, removeStoredToken } from './auth-storage';

export const authInterceptor: HttpInterceptorFn = (request, next) => {
  const token = getStoredToken();
  const isLoginRequest = request.url.endsWith('/api/auth/login');

  if (!token || isLoginRequest) {
    return next(request);
  }

  const router = inject(Router);
  return next(request.clone({ setHeaders: { Authorization: `Bearer ${token}` } })).pipe(
    catchError((error: HttpErrorResponse) => {
      if (error.status === 401) {
        removeStoredToken();
        void router.navigateByUrl('/login');
      }
      return throwError(() => error);
    })
  );
};
