import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { User } from '../auth/auth.models';
import { Company } from '../internship/internship.models';
import {
  CompanyPayload,
  CompanySupervisor,
  CompanySupervisorPayload,
  CreatedAccount,
  ImportResult,
  ProfessorAssignment,
  ProfessorPayload,
  StudentPayload
} from './university-management.models';

@Injectable({ providedIn: 'root' })
export class UniversityManagementService {
  private readonly http = inject(HttpClient);

  listStudents(): Observable<User[]> {
    return this.http.get<User[]>('/api/university/students');
  }

  createStudent(payload: StudentPayload): Observable<CreatedAccount> {
    return this.http.post<CreatedAccount>('/api/university/students', payload);
  }

  importStudents(file: File): Observable<ImportResult> {
    return this.importFile('/api/university/students/import', file);
  }

  listProfessors(): Observable<User[]> {
    return this.http.get<User[]>('/api/university/professors');
  }

  createProfessor(payload: ProfessorPayload): Observable<CreatedAccount> {
    return this.http.post<CreatedAccount>('/api/university/professors', payload);
  }

  importProfessors(file: File): Observable<ImportResult> {
    return this.importFile('/api/university/professors/import', file);
  }

  listCompanies(): Observable<Company[]> {
    return this.http.get<Company[]>('/api/university/companies');
  }

  createCompany(payload: CompanyPayload): Observable<Company> {
    return this.http.post<Company>('/api/university/companies', payload);
  }

  listCompanySupervisors(): Observable<CompanySupervisor[]> {
    return this.http.get<CompanySupervisor[]>('/api/university/company-supervisors');
  }

  createCompanySupervisor(payload: CompanySupervisorPayload): Observable<CreatedAccount> {
    return this.http.post<CreatedAccount>('/api/university/company-supervisors', payload);
  }

  listProfessorAssignments(): Observable<ProfessorAssignment[]> {
    return this.http.get<ProfessorAssignment[]>('/api/university/professor-assignments');
  }

  saveProfessorAssignment(assignment: ProfessorAssignment, professorId: number): Observable<unknown> {
    const payload = { studentId: assignment.student.id, professorId };
    if (assignment.id) {
      return this.http.put(`/api/university/professor-assignments/${assignment.id}`, payload);
    }
    return this.http.post('/api/university/professor-assignments', payload);
  }

  private importFile(url: string, file: File): Observable<ImportResult> {
    const form = new FormData();
    form.append('file', file);
    return this.http.post<ImportResult>(url, form);
  }
}
