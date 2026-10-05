import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { User } from '../auth/auth.models';
import { Company, CompanyRegistrationStatus } from '../internship/internship.models';

export interface CompanyRegistrationDetail {
 company: Company;
 supervisors: User[];
 reviewer?: User;
}
@Injectable({ providedIn: 'root' })
export class CompanyRegistrationService {
 private readonly http = inject(HttpClient);
 list(status: CompanyRegistrationStatus | '') {
  return this.http.get<Company[]>('/api/admin/company-registrations', { params: status ? { status } : {} });
 }
 detail(id: number) { return this.http.get<CompanyRegistrationDetail>(`/api/admin/company-registrations/${id}`); }
 approve(id: number) { return this.http.post<Company>(`/api/admin/company-registrations/${id}/approve`, {}); }
 reject(id: number, reason: string) { return this.http.post<Company>(`/api/admin/company-registrations/${id}/reject`, { reason }); }
}
