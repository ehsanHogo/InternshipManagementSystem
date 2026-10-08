import { HttpClient } from '@angular/common/http';
import { inject, Injectable, signal } from '@angular/core';
import { defer, Observable, tap } from 'rxjs';
import { AuthService } from '../auth/auth.service';

export interface Notification {
  id: number;
  type: string;
  title: string;
  message: string;
  isRead: boolean;
  createdAt: string;
  actionPath?: string;
}

@Injectable({ providedIn: 'root' })
export class NotificationService {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);
  private readonly count = signal(0);
  private countRequest = 0;
  readonly unreadCount = this.count.asReadonly();

  list(): Observable<Notification[]> {
    return this.http.get<Notification[]>('/api/notifications');
  }

  markViewed(notificationIds: number[]): Observable<unknown> {
    return this.http.post('/api/notifications/mark-viewed', { notificationIds });
  }

  refreshCount(): Observable<{ count: number }> {
    return defer(() => {
      const request = ++this.countRequest;
      const userId = this.auth.user()?.id;
      return this.http.get<{ count: number }>('/api/notifications/unread-count').pipe(
        tap(result => {
          if (request === this.countRequest && userId === this.auth.user()?.id) this.count.set(result.count);
        })
      );
    });
  }

  reset(): void {
    ++this.countRequest;
    this.count.set(0);
  }
}
