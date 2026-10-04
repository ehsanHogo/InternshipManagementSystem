import { User } from '../auth/auth.models';

export type InternshipCaseStatus =
  | 'DRAFT'
  | 'PENDING_UNIVERSITY_REVIEW'
  | 'PENDING_COMPANY_DETAILS'
  | 'PENDING_FINAL_APPROVAL'
  | 'READY_TO_START'
  | 'ACTIVE'
  | 'COMPLETED'
  | 'CANCELLED';

/** PrimeNG Tag severities used for status chips across the app. */
export type StatusTagSeverity = 'secondary' | 'info' | 'success' | 'warn' | 'danger' | 'contrast';

export interface Company {
  id: number;
  name: string;
  nationalId: string;
  economicCode: string;
  website?: string;
  phone?: string;
  email?: string;
  address?: string;
  isApproved: boolean;
}

export interface AcceptedOpportunityApplication {
  id: number;
  status: "ACCEPTED";
  opportunity: {
    id: number;
    title: string;
    description: string;
    workField: string;
    location: string;
    company: Company;
  };
}

export interface InternshipPreference {
  id: number;
  priority: number;
  opportunityApplicationId: number;
  application: AcceptedOpportunityApplication;
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
  cancellationComment?: string;
  cancelledAt?: string;
  companyDetailsRevisionComment?: string;
  activatedAt?: string;
  finalReport?: FinalReport;
  weeklyReportCount: number;
  approvedReportCount: number;
  companyApprovedReportCount: number;
  canSubmitCompanyEvaluation: boolean;
  weeklyReports: WeeklyReport[];
  companyEvaluation?: CompanyEvaluation;
  finalResult?: ProfessorFinalResult;
  professorComment?: string;
  completedAt?: string;
}

export interface FileMetadata {
  id: number;
  originalName: string;
  uploadedAt: string;
}

export type FinalReportStatus = 'SUBMITTED' | 'REVISION_REQUESTED' | 'APPROVED';

export const finalReportStatusLabels: Record<FinalReportStatus, string> = {
  SUBMITTED: 'در انتظار بررسی استاد',
  REVISION_REQUESTED: 'نیازمند اصلاح',
  APPROVED: 'تأیید شده'
};

export function finalReportStatusSeverity(status: FinalReportStatus): StatusTagSeverity {
  switch (status) {
    case 'APPROVED':
      return 'success';
    case 'REVISION_REQUESTED':
      return 'danger';
    default:
      return 'info';
  }
}

export interface FinalReport {
  id: number;
  internshipCaseId: number;
  currentFileId: number;
  currentFile: FileMetadata;
  status: FinalReportStatus;
  reviewComment: string | null;
  submittedAt: string;
  reviewedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export type CompanyPlacement = Omit<InternshipPreference, 'priority'>;

export interface CompanyInternshipCase extends Omit<InternshipCase, 'preferences' | 'selectedPreference'> {
  preferences: CompanyPlacement[];
  selectedPreference?: CompanyPlacement;
}

export type WeeklyReviewStatus = 'PENDING' | 'APPROVED' | 'REVISION_REQUESTED';
export type WeeklyReportStatus = 'DRAFT' | 'SUBMITTED' | 'REVISION_REQUESTED' | 'APPROVED';
export const weeklyReportStatusLabels: Record<WeeklyReportStatus, string> = {
  DRAFT: 'پیش‌نویس', SUBMITTED: 'در انتظار بررسی', REVISION_REQUESTED: 'نیازمند اصلاح', APPROVED: 'تأیید شده'
};
export const weeklyReviewStatusLabels: Record<WeeklyReviewStatus, string> = {
  PENDING: 'در انتظار بررسی', APPROVED: 'تأیید شده', REVISION_REQUESTED: 'نیازمند اصلاح'
};

export function weeklyReportStatusSeverity(status: WeeklyReportStatus): StatusTagSeverity {
  switch (status) {
    case 'APPROVED':
      return 'success';
    case 'REVISION_REQUESTED':
      return 'danger';
    case 'DRAFT':
      return 'secondary';
    default:
      return 'info';
  }
}

export function weeklyReviewStatusSeverity(status: WeeklyReviewStatus): StatusTagSeverity {
  switch (status) {
    case 'APPROVED':
      return 'success';
    case 'REVISION_REQUESTED':
      return 'danger';
    default:
      return 'warn';
  }
}

export interface WeeklyReport {
  id: number;
  internshipCaseId: number;
  weekNumber: number;
  startDate: string;
  endDate: string;
  activityDescription: string;
  submittedAt: string | null;
  status: WeeklyReportStatus;
  companyReviewStatus: WeeklyReviewStatus;
  companyReviewComment: string | null;
  companyReviewedAt: string | null;
  professorReviewStatus: WeeklyReviewStatus;
  professorReviewComment: string | null;
  professorReviewedAt: string | null;
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
  approvedWeeklyReportCount: number;
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
  finalReport?: FinalReport;
  weeklyReportCount: number;
  approvedWeeklyReportCount: number;
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

export interface UniversityPlacementApprovalPayload {
  preferenceId: number;
  letterNumber: string;
  letterDate: string;
}

export interface UniversityReviewCancellationPayload {
  comment: string;
}

export interface PlacementDetailsPayload {
  internshipSubject: string;
  startDate: string;
  workplaceAddress: string;
  workplacePhone: string;
}

export const internshipStatusLabels: Record<InternshipCaseStatus, string> = {
  DRAFT: 'پیش‌نویس',
  PENDING_UNIVERSITY_REVIEW: 'در انتظار بررسی آموزش',
  PENDING_COMPANY_DETAILS: 'در انتظار اصلاح/ثبت اطلاعات توسط شرکت',
  PENDING_FINAL_APPROVAL: 'در انتظار تأیید نهایی آموزش',
  READY_TO_START: 'آماده شروع کارآموزی',
  ACTIVE: 'در حال انجام کارآموزی',
  COMPLETED: 'تکمیل شده',
  CANCELLED: 'لغو شده'
};

export function internshipStatusSeverity(status: InternshipCaseStatus): StatusTagSeverity {
  switch (status) {
    case 'PENDING_UNIVERSITY_REVIEW':
    case 'PENDING_COMPANY_DETAILS':
      return 'info';
    case 'PENDING_FINAL_APPROVAL':
    case 'READY_TO_START':
    case 'ACTIVE':
      return 'success';
    case 'COMPLETED':
      return 'contrast';
    case 'CANCELLED':
      return 'danger';
    default:
      return 'secondary';
  }
}

export interface PreferencePayload {
  priority: number;
  opportunityApplicationId: number;
}
