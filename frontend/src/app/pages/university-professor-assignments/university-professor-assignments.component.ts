import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { forkJoin, finalize } from 'rxjs';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';

import { User } from '../../auth/auth.models';
import { ProfessorAssignment } from '../../university-management/university-management.models';
import { UniversityManagementService } from '../../university-management/university-management.service';

@Component({
  selector: 'app-university-professor-assignments',
  imports: [FormsModule, ButtonModule, SelectModule, TableModule],
  templateUrl: './university-professor-assignments.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityProfessorAssignmentsComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);

  readonly assignments = signal<ProfessorAssignment[]>([]);
  readonly professors = signal<User[]>([]);
  readonly loading = signal(true);
  readonly savingStudentId = signal<number | null>(null);
  readonly selectedProfessors: Record<number, number | null> = {};

  constructor() { this.load(); }

  selectionChanged(studentId: number, professorId: number | null): void {
    this.selectedProfessors[studentId] = professorId;
  }

  save(assignment: ProfessorAssignment): void {
    const professorId = this.selectedProfessors[assignment.student.id];
    if (!professorId) return;
    this.savingStudentId.set(assignment.student.id);
    this.management.saveProfessorAssignment(assignment, professorId)
      .pipe(finalize(() => this.savingStudentId.set(null))).subscribe({
        next: () => {
          this.messages.add({ severity: 'success', summary: 'ثبت شد', detail: 'استاد فعلی دانشجو به‌روزرسانی شد.' });
          this.load();
        },
        error: (error: HttpErrorResponse) => this.showError(error)
      });
  }

  private load(): void {
    this.loading.set(true);
    forkJoin({ assignments: this.management.listProfessorAssignments(), professors: this.management.listProfessors() }).subscribe({
      next: ({ assignments, professors }) => {
        this.assignments.set(assignments);
        this.professors.set(professors);
        for (const assignment of assignments) this.selectedProfessors[assignment.student.id] = assignment.professor?.id ?? null;
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => { this.loading.set(false); this.showError(error); }
    });
  }

  private showError(error: HttpErrorResponse): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: error.status === 0 ? 'ارتباط با سرور برقرار نشد.' : (error.error?.error || 'ثبت تخصیص استاد ناموفق بود.') });
  }
}
