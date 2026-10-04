import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { Company } from '../internship/internship.models';
import { ApprovalCompanyDetail, EligibleCompany, PublicApprovedCompany } from './company-approval.models';

@Injectable({ providedIn: 'root' })
export class CompanyApprovalService {
  private readonly http = inject(HttpClient);

  listEligible(): Observable<EligibleCompany[]> {
    return this.http.get<EligibleCompany[]>('/api/university/companies/eligible-for-approval');
  }

  listApproved(): Observable<Company[]> {
    return this.http.get<Company[]>('/api/university/companies/approved');
  }

  getCompany(id: number): Observable<ApprovalCompanyDetail> {
    return this.http.get<ApprovalCompanyDetail>(`/api/university/companies/${id}`);
  }

  approve(id: number): Observable<Company> {
    return this.http.post<Company>(`/api/university/companies/${id}/approve`, null);
  }

  listStudentApproved(): Observable<PublicApprovedCompany[]> {
    return this.http.get<PublicApprovedCompany[]>('/api/student/approved-companies');
  }
}
