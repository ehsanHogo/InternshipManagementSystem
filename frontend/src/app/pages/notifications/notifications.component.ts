import { afterNextRender, Component, DestroyRef, inject, Injector, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router, RouterLink, UrlTree } from '@angular/router';
import { switchMap } from 'rxjs';
import { Notification, NotificationService } from '../../notifications/notification.service';
import { JalaliDatePipe } from '../../shared/jalali-date/jalali-date.pipe';

@Component({
  selector: 'app-notifications',
  imports: [RouterLink, JalaliDatePipe],
  templateUrl: './notifications.component.html',
  styleUrl: './notifications.component.scss'
})
export class NotificationsComponent {
  private readonly service = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly router = inject(Router);
  readonly notifications = signal<Notification[]>([]);
  readonly loading = signal(true);
  readonly error = signal('');

  actionLink(path: string): UrlTree {
    return this.router.parseUrl(path);
  }

  constructor() {
    this.service.list().pipe(takeUntilDestroyed()).subscribe({
      next: notifications => {
        this.notifications.set(notifications);
        this.loading.set(false);
        // A render must complete before acknowledging this exact list snapshot.
        afterNextRender(() => {
          if (this.destroyRef.destroyed) return;
          const ids = notifications.filter(item => !item.isRead).map(item => item.id);
          if (!ids.length) return;
          this.service.markViewed(ids).pipe(
            switchMap(() => {
              this.notifications.update(items => items.map(item => ids.includes(item.id) ? { ...item, isRead: true } : item));
              return this.service.refreshCount();
            }),
            takeUntilDestroyed(this.destroyRef)
          ).subscribe({ error: () => this.error.set('به‌روزرسانی وضعیت اعلان‌ها انجام نشد. صفحه را دوباره باز کنید.') });
        }, { injector: this.injector });
      },
      error: () => {
        this.loading.set(false);
        this.error.set('دریافت اعلان‌ها انجام نشد. صفحه را دوباره باز کنید.');
      }
    });
  }
}
