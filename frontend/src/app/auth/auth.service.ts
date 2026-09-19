import { HttpClient } from '@angular/common/http';
import { inject, Injectable, signal } from '@angular/core';
import { catchError, firstValueFrom, map, Observable, of, tap } from 'rxjs';

import { getStoredToken, removeStoredToken, storeToken } from './auth-storage';
import { LoginResponse, User } from './auth.models';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly currentUser = signal<User | null>(null);

  readonly user = this.currentUser.asReadonly();

  login(email: string, password: string): Observable<User> {
    return this.http.post<LoginResponse>('/api/auth/login', { email, password }).pipe(
      tap((response) => {
        storeToken(response.token);
        this.currentUser.set(response.user);
      }),
      map((response) => response.user)
    );
  }

  logout(): void {
    removeStoredToken();
    this.currentUser.set(null);
  }

  getCurrentUser(): User | null {
    return this.currentUser();
  }

  isAuthenticated(): boolean {
    return getStoredToken() !== null && this.currentUser() !== null;
  }

  async initialize(): Promise<void> {
    await firstValueFrom(this.restoreSession());
  }

  private restoreSession(): Observable<User | null> {
    if (!getStoredToken()) {
      this.currentUser.set(null);
      return of(null);
    }

    return this.http.get<User>('/api/auth/me').pipe(
      tap((user) => this.currentUser.set(user)),
      catchError(() => {
        this.logout();
        return of(null);
      })
    );
  }
}
