import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';

import {
  AcceptedOpportunityApplication,
  CompanyEvaluation,
  CompanyEvaluationPayload,
  PlacementDetailsPayload,
  InternshipCase,
  CompanyInternshipCase,
  InternshipCaseStatus,
  UniversityPlacementApprovalPayload,
  UniversityReviewCancellationPayload,
  FinalReport,
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

  listStudentHistoricalCases(): Observable<InternshipCase[]> {
    return this.http.get<InternshipCase[]>('/api/student/internship-cases/history');
  }

  getStudentHistoricalCase(id: number): Observable<InternshipCase> {
    return this.http.get<InternshipCase>(`/api/student/internship-cases/history/${id}`);
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

  replacePreferences(opportunityApplicationIds: number[]): Observable<InternshipCase> {
    return this.http.put<InternshipCase>("/api/student/internship-case/preferences", { opportunityApplicationIds });
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

  approveUniversityPlacement(id: number, payload: UniversityPlacementApprovalPayload): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/approve-placement`, payload);
  }

  cancelUniversityReview(id: number, payload: UniversityReviewCancellationPayload): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/cancel`, payload);
  }

  approvePlacementDetails(id: number): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/final-approve`, {});
  }

  requestPlacementCorrection(id: number, comment: string): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/request-placement-correction`, { comment });
  }

  activateUniversityCase(id: number): Observable<InternshipCase> {
    return this.http.post<InternshipCase>(`/api/university/internship-cases/${id}/activate`, null);
  }

  listCompanyCases(): Observable<CompanyInternshipCase[]> {
    return this.http.get<CompanyInternshipCase[]>('/api/company/internship-cases');
  }

  listPendingCompanyDetailsCases(): Observable<CompanyInternshipCase[]> {
    return this.http.get<CompanyInternshipCase[]>('/api/company/internship-cases/pending-details');
  }

  getCompanyCase(id: number): Observable<CompanyInternshipCase> {
    return this.http.get<CompanyInternshipCase>(`/api/company/internship-cases/${id}`);
  }

  submitPlacementDetails(id: number, payload: PlacementDetailsPayload): Observable<CompanyInternshipCase> {
    return this.http.post<CompanyInternshipCase>(`/api/company/internship-cases/${id}/placement-details`, payload);
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

  uploadFinalReport(file: File): Observable<FinalReport> {
    const form = new FormData();
    form.append('file', file);
    return this.http.post<FinalReport>('/api/student/internship-case/final-report', form);
  }

  getStudentFinalReport(): Observable<FinalReport | null> {
    return this.http.get<FinalReport | null>('/api/student/internship-case/final-report');
  }

  getProfessorFinalReport(caseId: number): Observable<FinalReport | null> {
    return this.http.get<FinalReport | null>(`/api/professor/internship-cases/${caseId}/final-report`);
  }

  reviewFinalReport(caseId: number, action: 'approve' | 'request-revision', comment?: string): Observable<FinalReport> {
    return this.http.post<FinalReport>(`/api/professor/internship-cases/${caseId}/final-report/${action}`, { comment });
  }

  downloadFile(id: number): Observable<Blob> {
    return this.http.get(`/api/files/${id}/download`, { responseType: 'blob' });
  }

  reviewCompanyWeeklyReport(caseId: number, reportId: number, action: 'approve' | 'request-revision', comment?: string): Observable<WeeklyReport> {
    return this.http.post<WeeklyReport>(`/api/company/internship-cases/${caseId}/weekly-reports/${reportId}/${action}`, { comment });
  }

  reviewProfessorWeeklyReport(caseId: number, reportId: number, action: 'approve' | 'request-revision', comment?: string): Observable<WeeklyReport> {
    return this.http.post<WeeklyReport>(`/api/professor/internship-cases/${caseId}/weekly-reports/${reportId}/${action}`, { comment });
  }

  submitWeeklyReport(reportId: number): Observable<WeeklyReport> {
    return this.http.post<WeeklyReport>(`/api/student/internship-case/weekly-reports/${reportId}/submit`, {});
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
