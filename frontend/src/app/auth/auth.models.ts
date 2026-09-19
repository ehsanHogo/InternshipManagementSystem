export type UserRole =
  | 'STUDENT'
  | 'PROFESSOR'
  | 'COMPANY_SUPERVISOR'
  | 'UNIVERSITY_SUPERVISOR'
  | 'ADMIN';

export interface User {
  id: number;
  fullName: string;
  email: string;
  role: UserRole;
  studentNumber?: string;
  major?: string;
  companyId?: number;
}

export interface LoginResponse {
  token: string;
  user: User;
}
