import { User } from '../auth/auth.models';
import { Company } from '../internship/internship.models';

export interface CreatedAccount {
  user: User;
  temporaryPassword: string;
}

export interface StudentPayload {
  fullName: string;
  email: string;
  studentNumber: string;
  major: string;
}

export interface ProfessorPayload {
  fullName: string;
  email: string;
}

export interface CompanySupervisor extends User {
  company?: Company;
}

export interface ImportRowResult {
  row: number;
  name?: string;
  email?: string;
  status: 'CREATED' | 'SKIPPED' | 'FAILED';
  temporaryPassword?: string;
  message?: string;
}

export interface ImportResult {
  created: number;
  skipped: number;
  results: ImportRowResult[];
}

export interface ProfessorAssignment {
  id: number;
  student: User;
  professor?: User;
}
