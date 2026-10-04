import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { catchError, forkJoin, of } from 'rxjs';
import { RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { AuthService } from '../../auth/auth.service';
import {
  InternshipCase,
  InternshipCaseStatus,
  finalReportStatusLabels,
  finalReportStatusSeverity,
  internshipStatusLabels,
  internshipStatusSeverity,
  professorFinalResultLabels
} from '../../internship/internship.models';
import { InternshipService } from '../../internship/internship.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';
import { PersianDigitsPipe } from '../../shared/persian-digits.pipe';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink, ButtonModule, CardModule, TagModule, PersianDigitsPipe, JalaliDatePipe],
  templateUrl: './dashboard.component.html',
  styleUrl: './dashboard.component.scss'
})
export class DashboardComponent {
  private readonly auth = inject(AuthService);
  private readonly internshipService = inject(InternshipService);

  readonly user = this.auth.user;
  readonly internshipStatus = signal<InternshipCaseStatus | null>(null);
  readonly internshipCase = signal<InternshipCase | null>(null);
  readonly historicalCase = signal<InternshipCase | null>(null);
  readonly hasPassedCase = signal(false);
  readonly statusLoading = signal(false);
  readonly statusLoadFailed = signal(false);

  constructor() {
    if (this.auth.getCurrentUser()?.role === 'STUDENT') {
      this.loadStudentStatus();
    }
  }

  loadStudentStatus(): void {
    this.statusLoading.set(true);
    this.statusLoadFailed.set(false);
    forkJoin({
      current: this.internshipService.getCurrentCase().pipe(catchError((error: HttpErrorResponse) => {
        if (error.status === 404) return of(null);
        throw error;
      })),
      history: this.internshipService.listStudentHistoricalCases()
    }).subscribe({
      next: ({ current: internshipCase, history }) => {
        this.historicalCase.set(history[0] ?? null);
        this.hasPassedCase.set(history.some(item => item.status === 'PASSED'));
        this.internshipCase.set(internshipCase);
        this.internshipStatus.set(internshipCase?.status ?? null);
        this.statusLoading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        if (error.status === 404) {
          this.internshipCase.set(null);
          this.internshipStatus.set(null);
        } else {
          this.statusLoadFailed.set(true);
        }
        this.statusLoading.set(false);
      }
    });
  }

  readonly internshipStatusSeverity = internshipStatusSeverity;
  readonly finalReportStatusLabels = finalReportStatusLabels;
  readonly finalReportStatusSeverity = finalReportStatusSeverity;

  statusLabel(status: InternshipCaseStatus | null): string {
    return status ? internshipStatusLabels[status] : 'درخواستی ثبت نشده است';
  }

  finalResultLabel(internshipCase: InternshipCase): string {
    return internshipCase.finalResult ? professorFinalResultLabels[internshipCase.finalResult] : '—';
  }
}
