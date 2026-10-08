import { afterNextRender, Component, computed, DestroyRef, ElementRef, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { Drawer } from 'primeng/drawer';
import { TagModule } from 'primeng/tag';
import { filter } from 'rxjs';

import { NotificationService } from '../notifications/notification.service';
import { PersianDigitsPipe } from '../shared/persian-digits.pipe';
import { UserRole } from '../auth/auth.models';
import { AuthService } from '../auth/auth.service';

const roleLabels: Record<UserRole, string> = {
  STUDENT: 'دانشجو',
  PROFESSOR: 'استاد',
  COMPANY_SUPERVISOR: 'سرپرست شرکت',
  UNIVERSITY_SUPERVISOR: 'مسئول آموزش',
  ADMIN: 'مدیر سامانه'
};

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, ButtonModule, Drawer, TagModule, PersianDigitsPipe],
  templateUrl: './app-shell.component.html',
  styleUrl: './app-shell.component.scss'
})
export class AppShellComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  private readonly notifications = inject(NotificationService);
  readonly unreadCount = this.notifications.unreadCount;

  readonly shellBodyRef = viewChild.required<ElementRef<HTMLElement>>('shellBody');

  readonly user = this.auth.user;
  readonly restrictedUniversity = computed(() => this.user()?.role === 'UNIVERSITY_SUPERVISOR' && this.user()?.verificationStatus !== 'APPROVED');
  readonly restrictedCompany = computed(() => this.user()?.role === 'COMPANY_SUPERVISOR' && this.user()?.companyRegistrationStatus !== 'APPROVED');
  readonly navOpen = signal(false);
  readonly topbarElevated = signal(false);

  constructor() {
    this.refreshNotificationCount();
    this.destroyRef.onDestroy(() => this.notifications.reset());
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed()
      )
      .subscribe(() => {
        this.navOpen.set(false);
        this.refreshNotificationCount();
        queueMicrotask(() => {
          this.shellBodyRef().nativeElement.scrollTop = 0;
          this.syncTopbarElevation();
        });
      });

    afterNextRender(() => {
      const shellBody = this.shellBodyRef().nativeElement;

      this.syncTopbarElevation();
      shellBody.addEventListener('scroll', this.syncTopbarElevation, { passive: true });
      this.destroyRef.onDestroy(() => shellBody.removeEventListener('scroll', this.syncTopbarElevation));
    });
  }

  private refreshNotificationCount(): void {
    this.notifications.refreshCount().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ error: () => this.notifications.reset() });
  }

  private syncTopbarElevation = (): void => {
    const shellBody = this.shellBodyRef()?.nativeElement;
    this.topbarElevated.set((shellBody?.scrollTop ?? 0) > 2);
  };

  roleLabel(role: UserRole): string {
    return roleLabels[role];
  }

  openNav(): void {
    this.navOpen.set(true);
  }

  onNavVisibleChange(visible: boolean): void {
    this.navOpen.set(visible);
  }

  logout(): void {
    this.auth.logout();
    void this.router.navigateByUrl('/login');
  }
}
