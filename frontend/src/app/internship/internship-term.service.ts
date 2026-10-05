import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { InternshipTerm, InternshipTermDetail, InternshipTermType } from './internship-term.models';

@Injectable({ providedIn: 'root' })
export class InternshipTermService {
  private readonly http = inject(HttpClient);

  current(): Observable<InternshipTerm | null> {
    return this.http.get<InternshipTerm | null>('/api/student/internship-term');
  }

  list(): Observable<InternshipTerm[]> {
    return this.http.get<InternshipTerm[]>('/api/university/internship-terms');
  }

  detail(id: number): Observable<InternshipTermDetail> {
    return this.http.get<InternshipTermDetail>(`/api/university/internship-terms/${id}`);
  }

  create(academicYear: number, termType: InternshipTermType): Observable<InternshipTerm> {
    return this.http.post<InternshipTerm>('/api/university/internship-terms', { academicYear, termType });
  }

  close(id: number): Observable<InternshipTerm> {
    return this.http.post<InternshipTerm>(`/api/university/internship-terms/${id}/close`, {});
  }
}
