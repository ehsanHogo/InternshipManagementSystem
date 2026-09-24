import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import {
  CompanyAccountProfile,
  CompanyRegistrationPayload,
  CompanyRegistrationResponse
} from './company-account.models';

@Injectable({ providedIn: 'root' })
export class CompanyAccountService {
  private readonly http = inject(HttpClient);

  register(payload: CompanyRegistrationPayload): Observable<CompanyRegistrationResponse> {
    return this.http.post<CompanyRegistrationResponse>('/api/auth/company-register', payload);
  }

  getProfile(): Observable<CompanyAccountProfile> {
    return this.http.get<CompanyAccountProfile>('/api/company/profile');
  }
}
