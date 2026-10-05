import { InternshipCase } from './internship.models';

export type InternshipTermType = 'FIRST' | 'SECOND' | 'SUMMER';

export interface InternshipTerm {
  id: number;
  academicYear: number;
  termType: InternshipTermType;
  status: 'OPEN' | 'CLOSED';
  openedAt: string;
  closedAt: string | null;
  createdBy: number;
  closedBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface InternshipTermDetail {
  term: InternshipTerm;
  cases: InternshipCase[];
}

export const internshipTermTypeLabels: Record<InternshipTermType, string> = {
  FIRST: 'نیمسال اول', SECOND: 'نیمسال دوم', SUMMER: 'تابستان'
};

export function internshipTermLabel(term: InternshipTerm | null | undefined): string {
  return term ? `${internshipTermTypeLabels[term.termType]} ${term.academicYear}` : 'ثبت نشده (پرونده قدیمی)';
}
