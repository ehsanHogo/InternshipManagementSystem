import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import {
  CompanyOpportunity,
  OpportunityApplication,
  OpportunityPayload,
  StudentOpportunity
} from './opportunity.models';

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

  apply(id: number, resume: File): Observable<OpportunityApplication> {
    const body = new FormData();
    body.append('resume', resume);
    return this.http.post<OpportunityApplication>(`/api/student/opportunities/${id}/apply`, body);
  }

  listStudentApplications(): Observable<OpportunityApplication[]> {
    return this.http.get<OpportunityApplication[]>('/api/student/opportunity-applications');
  }

  getStudentApplication(id: number): Observable<OpportunityApplication> {
    return this.http.get<OpportunityApplication>(`/api/student/opportunity-applications/${id}`);
  }

  listCompanyApplications(opportunityId: number): Observable<OpportunityApplication[]> {
    return this.http.get<OpportunityApplication[]>(`/api/company/opportunities/${opportunityId}/applications`);
  }

  getCompanyApplication(id: number): Observable<OpportunityApplication> {
    return this.http.get<OpportunityApplication>(`/api/company/opportunity-applications/${id}`);
  }

  reviewApplication(id: number, decision: 'accept' | 'reject', companyComment?: string): Observable<OpportunityApplication> {
    return this.http.post<OpportunityApplication>(`/api/company/opportunity-applications/${id}/${decision}`, {
      companyComment: companyComment?.trim() || null
    });
  }

  downloadResume(fileId: number): Observable<Blob> {
    return this.http.get(`/api/files/${fileId}/download`, { responseType: 'blob' });
  }
}
