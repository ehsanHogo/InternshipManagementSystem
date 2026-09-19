import { User } from '../auth/auth.models';

export type InternshipCaseStatus =
  | 'DRAFT'
  | 'PENDING_UNIVERSITY_APPROVAL'
  | 'PENDING_COMPANY_APPROVAL'
  | 'COMPANY_APPROVED'
  | 'UNIVERSITY_APPROVED'
  | 'ACTIVE'
  | 'COMPLETED';

export interface Company {
  id: number;
  name: string;
  website?: string;
  phone?: string;
  email?: string;
  address?: string;
  isApproved: boolean;
}

export interface InternshipPreference {
  id: number;
  priority: number;
  companyId?: number;
  company?: Company;
  proposedCompanyName?: string;
  proposedWebsite?: string;
  proposedPhone?: string;
  proposedEmail?: string;
  proposedSupervisorName?: string;
  city: string;
  workField: string;
}

export interface InternshipCase {
  id: number;
  status: InternshipCaseStatus;
  passedCredits: number | null;
  mobile: string | null;
  student: User;
  professor: User;
  preferences: InternshipPreference[];
  selectedPreferenceId?: number;
  selectedPreference?: InternshipPreference;
  companySupervisorId?: number;
  companySupervisor?: User;
  letterNumber?: string;
  letterDate?: string;
  internshipSubject?: string;
  startDate?: string;
  workplaceAddress?: string;
  workplacePhone?: string;
  createdAt: string;
  updatedAt: string;
  submittedAt?: string;
  companyConfirmedAt?: string;
  universityApprovedAt?: string;
  activatedAt?: string;
  finalReport?: FileMetadata;
  weeklyReportCount: number;
  confirmedReportCount: number;
  canSubmitCompanyEvaluation: boolean;
  finalResult?: ProfessorFinalResult;
  professorComment?: string;
  completedAt?: string;
}

export interface FileMetadata {
  id: number;
  originalName: string;
  uploadedAt: string;
}

export interface WeeklyReport {
  id: number;
  internshipCaseId: number;
  weekNumber: number;
  startDate: string;
  endDate: string;
  activityDescription: string;
  submittedAt: string;
  isConfirmed: boolean;
  supervisorComment?: string;
  confirmedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface WeeklyReportPayload {
  weekNumber: number;
  startDate: string;
  endDate: string;
  activityDescription: string;
}

export type EvaluationRating = 'EXCELLENT' | 'GOOD' | 'AVERAGE' | 'WEAK' | 'FAILED';

export interface CompanyEvaluation {
  id: number;
  internshipCaseId: number;
  companySupervisorId: number;
  attendanceRating: EvaluationRating;
  participationRating: EvaluationRating;
  learningRating: EvaluationRating;
  interestRating: EvaluationRating;
  persistenceRating: EvaluationRating;
  suggestionRating: EvaluationRating;
  resourceUsageRating: EvaluationRating;
  reportQualityRating: EvaluationRating;
  projectPerformanceRating: EvaluationRating;
  leaveDays: number;
  absenceDays: number;
  suggestions?: string;
  submittedAt: string;
}

export type CompanyEvaluationPayload = Omit<CompanyEvaluation, 'id' | 'internshipCaseId' | 'companySupervisorId' | 'submittedAt'>;

export const evaluationRatingLabels: Record<EvaluationRating, string> = {
  EXCELLENT: 'عالی',
  GOOD: 'خوب',
  AVERAGE: 'متوسط',
  WEAK: 'ضعیف',
  FAILED: 'مردود'
};

export type ProfessorFinalResult = 'EXCELLENT' | 'GOOD' | 'FAILED';

export const professorFinalResultLabels: Record<ProfessorFinalResult, string> = {
  EXCELLENT: 'عالی',
  GOOD: 'خوب',
  FAILED: 'مردود'
};

export interface ProfessorCaseListItem {
  caseId: number;
  student: User;
  company: string;
  internshipSubject?: string;
  status: InternshipCaseStatus;
  weeklyReportCount: number;
  confirmedWeeklyReportCount: number;
  hasCompanyEvaluation: boolean;
  hasFinalReport: boolean;
  canProfessorComplete: boolean;
  finalResult?: ProfessorFinalResult;
}

export interface ProfessorStudentInformation {
  fullName: string;
  studentNumber?: string;
  major?: string;
  passedCredits: number | null;
  mobile?: string;
}

export interface ProfessorInternshipInformation {
  company: string;
  internshipSubject?: string;
  startDate?: string;
  workplaceAddress?: string;
  workplacePhone?: string;
  companySupervisor?: User;
  status: InternshipCaseStatus;
}

export interface ProfessorCompanyEvaluation extends CompanyEvaluation {
  companySupervisor?: User;
}

export interface ProfessorCaseDetail {
  caseId: number;
  student: ProfessorStudentInformation;
  internship: ProfessorInternshipInformation;
  weeklyReports: WeeklyReport[];
  companyEvaluation?: ProfessorCompanyEvaluation;
  finalReport?: FileMetadata;
  weeklyReportCount: number;
  confirmedWeeklyReportCount: number;
  hasCompanyEvaluation: boolean;
  hasFinalReport: boolean;
  canProfessorComplete: boolean;
  finalResult?: ProfessorFinalResult;
  professorComment?: string;
  completedAt?: string;
}

export interface ProfessorCompletionPayload {
  result: ProfessorFinalResult;
  comment?: string;
}

export interface SendToCompanyPayload {
  preferenceId: number;
  companySupervisorId: number;
  letterNumber: string;
  letterDate: string;
}

export interface CompanyConfirmationPayload {
  internshipSubject: string;
  startDate: string;
  workplaceAddress: string;
  workplacePhone: string;
}

export const internshipStatusLabels: Record<InternshipCaseStatus, string> = {
  DRAFT: 'پیش‌نویس',
  PENDING_UNIVERSITY_APPROVAL: 'در انتظار تأیید آموزش',
  PENDING_COMPANY_APPROVAL: 'در انتظار تأیید شرکت',
  COMPANY_APPROVED: 'تأیید شده توسط شرکت',
  UNIVERSITY_APPROVED: 'تأیید شده توسط آموزش',
  ACTIVE: 'کارآموزی فعال',
  COMPLETED: 'تکمیل شده'
};

export interface PreferencePayload {
  priority: number;
  companyId?: number;
  proposedCompanyName?: string;
  proposedWebsite?: string;
  proposedPhone?: string;
  proposedEmail?: string;
  proposedSupervisorName?: string;
  city: string;
  workField: string;
}
