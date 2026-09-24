import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { CompanyOpportunity, OpportunityPayload, StudentOpportunity } from './opportunity.models';

@Injectable({ providedIn: 'root' })
export class OpportunityService {
  private readonly http = inject(HttpClient);

  listCompany(): Observable<CompanyOpportunity[]> {
    return this.http.get<CompanyOpportunity[]>('/api/company/opportunities');
  }

  getCompany(id: number): Observable<CompanyOpportunity> {
    return this.http.get<CompanyOpportunity>(`/api/company/opportunities/${id}`);
  }

  create(payload: OpportunityPayload): Observable<CompanyOpportunity> {
    return this.http.post<CompanyOpportunity>('/api/company/opportunities', payload);
  }

  update(id: number, payload: OpportunityPayload): Observable<CompanyOpportunity> {
    return this.http.put<CompanyOpportunity>(`/api/company/opportunities/${id}`, payload);
  }

  close(id: number): Observable<CompanyOpportunity> {
    return this.http.post<CompanyOpportunity>(`/api/company/opportunities/${id}/close`, {});
  }

  listStudent(): Observable<StudentOpportunity[]> {
    return this.http.get<StudentOpportunity[]>('/api/student/opportunities');
  }

  getStudent(id: number): Observable<StudentOpportunity> {
    return this.http.get<StudentOpportunity>(`/api/student/opportunities/${id}`);
  }
}
