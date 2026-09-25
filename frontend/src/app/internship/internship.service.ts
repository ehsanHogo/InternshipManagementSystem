import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import { User } from '../auth/auth.models';
import {
  AcceptedOpportunityApplication,
  Company,
  CompanyEvaluation,
  CompanyEvaluationPayload,
  CompanyConfirmationPayload,
  InternshipCase,
  InternshipCaseStatus,
  InternshipPreference,
  PreferencePayload,
  SendToCompanyPayload,
  FileMetadata,
  ProfessorCaseDetail,
  ProfessorCaseListItem,
  ProfessorCompletionPayload,
  WeeklyReport,
  WeeklyReportPayload
} from './internship.models';

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

  listAcceptedOpportunityApplications(): Observable<AcceptedOpportunityApplication[]> {
    return this.http.get<AcceptedOpportunityApplication[]>("/api/student/accepted-opportunity-applications");
  }

  listCompanies(): Observable<Company[]> {
    return this.http.get<Company[]>('/api/companies');
  }

  replacePreferences(opportunityApplicationIds: number[]): Observable<InternshipCase> {
    return this.http.put<InternshipCase>("/api/student/internship-case/preferences", { opportunityApplicationIds });
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

  listUniversityCases(status?: InternshipCaseStatus): Observable<InternshipCase[]> {
    const options = status ? { params: { status } } : {};
    return this.http.get<InternshipCase[]>('/api/university/internship-cases', options);
  }

  getUniversityCase(id: number): Observable<InternshipCase> {
    return this.http.get<InternshipCase>(`/api/university/internship-cases/${id}`);
  }

  listCompanySupervisors(): Observable<User[]> {
    return this.http.get<User[]>('/api/university/company-supervisors');
  }

  sendToCompany(id: number, payload: SendToCompanyPayload): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/send-to-company`, payload);
  }

  approveUniversityCase(id: number): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/approve`, {});
  }

  activateUniversityCase(id: number): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/activate`, {});
  }

  listCompanyCases(): Observable<InternshipCase[]> {
    return this.http.get<InternshipCase[]>('/api/company/internship-cases');
  }

  getCompanyCase(id: number): Observable<InternshipCase> {
    return this.http.get<InternshipCase>(`/api/company/internship-cases/${id}`);
  }

  confirmCompanyCase(id: number, payload: CompanyConfirmationPayload): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/company/internship-cases/${id}/confirm`, payload);
  }

  listStudentWeeklyReports(): Observable<WeeklyReport[]> {
    return this.http.get<WeeklyReport[]>('/api/student/internship-case/weekly-reports');
  }

  createWeeklyReport(payload: WeeklyReportPayload): Observable<WeeklyReport> {
    return this.http.post<WeeklyReport>('/api/student/internship-case/weekly-reports', payload);
  }

  updateWeeklyReport(id: number, payload: WeeklyReportPayload): Observable<WeeklyReport> {
    return this.http.put<WeeklyReport>(`/api/student/internship-case/weekly-reports/${id}`, payload);
  }

  uploadFinalReport(file: File): Observable<FileMetadata> {
    const form = new FormData();
    form.append('file', file);
    return this.http.post<FileMetadata>('/api/student/internship-case/final-report', form);
  }

  downloadFile(id: number): Observable<Blob> {
    return this.http.get(`/api/files/${id}/download`, { responseType: 'blob' });
  }

  listCompanyWeeklyReports(caseId: number): Observable<WeeklyReport[]> {
    return this.http.get<WeeklyReport[]>(`/api/company/internship-cases/${caseId}/weekly-reports`);
  }

  confirmWeeklyReport(caseId: number, reportId: number, comment?: string): Observable<WeeklyReport> {
    return this.http.post<WeeklyReport>(
      `/api/company/internship-cases/${caseId}/weekly-reports/${reportId}/confirm`,
      { comment: comment?.trim() || null }
    );
  }

  getCompanyEvaluation(caseId: number): Observable<CompanyEvaluation> {
    return this.http.get<CompanyEvaluation>(`/api/company/internship-cases/${caseId}/evaluation`);
  }

  createCompanyEvaluation(caseId: number, payload: CompanyEvaluationPayload): Observable<CompanyEvaluation> {
    return this.http.post<CompanyEvaluation>(`/api/company/internship-cases/${caseId}/evaluation`, payload);
  }

  listProfessorCases(): Observable<ProfessorCaseListItem[]> {
    return this.http.get<ProfessorCaseListItem[]>('/api/professor/internship-cases');
  }

  getProfessorCase(id: number): Observable<ProfessorCaseDetail> {
    return this.http.get<ProfessorCaseDetail>(`/api/professor/internship-cases/${id}`);
  }

  completeProfessorCase(id: number, payload: ProfessorCompletionPayload): Observable<ProfessorCaseDetail> {
    return this.http.post<ProfessorCaseDetail>(`/api/professor/internship-cases/${id}/complete`, payload);
  }
}
