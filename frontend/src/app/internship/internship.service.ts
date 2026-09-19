import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { Company, InternshipCase, InternshipPreference, PreferencePayload } from './internship.models';

@Injectable({ providedIn: 'root' })
export class InternshipService {
  private readonly http = inject(HttpClient);

  getCurrentCase(): Observable<InternshipCase> {
    return this.http.get<InternshipCase>('/api/student/internship-case');
  }

  createCase(): Observable<InternshipCase> {
    return this.http.post<InternshipCase>('/api/student/internship-case', {});
  }

  updateCase(passedCredits: number | null, mobile: string): Observable<InternshipCase> {
    return this.http.put<InternshipCase>('/api/student/internship-case', { passedCredits, mobile });
  }

  listCompanies(): Observable<Company[]> {
    return this.http.get<Company[]>('/api/companies');
  }

  addPreference(payload: PreferencePayload): Observable<InternshipPreference> {
    return this.http.post<InternshipPreference>('/api/student/internship-case/preferences', payload);
  }

  updatePreference(id: number, payload: PreferencePayload): Observable<InternshipPreference> {
    return this.http.put<InternshipPreference>(`/api/student/internship-case/preferences/${id}`, payload);
  }

  deletePreference(id: number): Observable<void> {
    return this.http.delete<void>(`/api/student/internship-case/preferences/${id}`);
  }

  submitCase(): Observable<InternshipCase> {
    return this.http.post<InternshipCase>('/api/student/internship-case/submit', {});
  }
}
