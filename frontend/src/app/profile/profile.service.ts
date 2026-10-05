import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable, tap } from 'rxjs';

import { User } from '../auth/auth.models';
import { AuthService } from '../auth/auth.service';

export interface ProfileUpdate {
  fullName: string;
  phone: string;
  jobTitle?: string;
}

@Injectable({ providedIn: 'root' })
export class ProfileService {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);

  update(input: ProfileUpdate): Observable<User> {
    return this.http.put<User>('/api/profile', input).pipe(tap(user => this.auth.updateCachedUser(user)));
  }

  changePassword(currentPassword: string, newPassword: string): Observable<{ message: string }> {
    return this.http.post<{ message: string }>('/api/profile/change-password', { currentPassword, newPassword });
  }
}
