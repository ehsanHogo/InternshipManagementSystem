import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import {
  CompanyAccountProfile,
  CompanyProfileUpdatePayload,
  CompanyRegistrationPayload,
  CompanyRegistrationResponse
} from './company-account.models';

@Injectable({ providedIn: 'root' })
export class CompanyAccountService {
  private readonly http = inject(HttpClient);

  register(payload: CompanyRegistrationPayload): Observable<CompanyRegistrationResponse> {
    return this.http.post<CompanyRegistrationResponse>('/api/auth/company-register', payload);
  }

  updateProfile(payload: CompanyProfileUpdatePayload): Observable<CompanyAccountProfile> {
    return this.http.put<CompanyAccountProfile>('/api/company/profile', payload);
  }

  resubmit(): Observable<CompanyAccountProfile> {
    return this.http.post<CompanyAccountProfile>('/api/company/registration/resubmit', {});
  }

  getProfile(): Observable<CompanyAccountProfile> {
    return this.http.get<CompanyAccountProfile>('/api/company/profile');
  }
}
