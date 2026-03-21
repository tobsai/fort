/**
 * Google Calendar Integration — Event management with draft-first safety
 *
 * Event creation is Tier 2 (draft behavior). Querying is Tier 1 (auto).
 * All operations publish events on ModuleBus.
 */

import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import { BaseIntegration } from './base.js';

export interface CalendarConfig {
  enabled: boolean;
  clientId?: string;
  clientSecret?: string;
  refreshToken?: string;
  defaultCalendarId?: string;
}

export interface CalendarEvent {
  id: string;
  summary: string;
  description?: string;
  start: Date;
  end: Date;
  attendees: string[];
  calendarId: string;
  location?: string;
  status: string;
  htmlLink?: string;
}

export interface FreeTimeSlot {
  start: Date;
  end: Date;
  durationMinutes: number;
}

export class CalendarIntegration extends BaseIntegration {
  id = 'calendar';
  name = 'Google Calendar';

  private config: CalendarConfig;
  private tokenValid = false;

  constructor(bus: ModuleBus, permissions: PermissionManager, config: CalendarConfig) {
    super(bus, permissions);
    this.config = config;
  }

  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      this.status = 'disconnected';
      return;
    }

    if (!this.config.clientId || !this.config.clientSecret || !this.config.refreshToken) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: 'Missing OAuth2 credentials for Google Calendar.',
      });
      return;
    }

    try {
      this.tokenValid = await this.validateToken();
      this.status = this.tokenValid ? 'connected' : 'error';
      await this.publishEvent('integration.initialized', {
        integration: this.id,
        status: this.status,
      });
    } catch (err) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async listEvents(timeMin: Date, timeMax: Date, calendarId?: string): Promise<CalendarEvent[]> {
    this.ensureConfigured();

    const permission = this.checkPermission('query_calendar');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Calendar listEvents requires approval: ${permission.description}`);
    }

    const calendar = calendarId || this.config.defaultCalendarId || 'primary';
    await this.publishEvent('calendar.listEvents', {
      timeMin: timeMin.toISOString(),
      timeMax: timeMax.toISOString(),
      calendarId: calendar,
    });

    try {
      const params = new URLSearchParams({
        timeMin: timeMin.toISOString(),
        timeMax: timeMax.toISOString(),
        singleEvents: 'true',
        orderBy: 'startTime',
      });

      const response = await this.apiRequest<{ items?: Array<Record<string, unknown>> }>(
        `https://www.googleapis.com/calendar/v3/calendars/${encodeURIComponent(calendar)}/events?${params}`
      );

      const events = (response.items || []).map((item) => this.parseEvent(item, calendar));

      await this.publishEvent('calendar.listEvents.complete', {
        count: events.length,
        calendarId: calendar,
      });

      return events;
    } catch (err) {
      await this.publishEvent('calendar.listEvents.error', {
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async getEvent(eventId: string, calendarId?: string): Promise<CalendarEvent> {
    this.ensureConfigured();

    const permission = this.checkPermission('query_calendar');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Calendar getEvent requires approval: ${permission.description}`);
    }

    const calendar = calendarId || this.config.defaultCalendarId || 'primary';
    await this.publishEvent('calendar.getEvent', { eventId, calendarId: calendar });

    try {
      const raw = await this.apiRequest<Record<string, unknown>>(
        `https://www.googleapis.com/calendar/v3/calendars/${encodeURIComponent(calendar)}/events/${eventId}`
      );

      const event = this.parseEvent(raw, calendar);
      await this.publishEvent('calendar.getEvent.complete', { eventId });
      return event;
    } catch (err) {
      await this.publishEvent('calendar.getEvent.error', {
        eventId,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async createEvent(
    summary: string,
    start: Date,
    end: Date,
    attendees?: string[],
    calendarId?: string
  ): Promise<CalendarEvent> {
    this.ensureConfigured();

    // Tier 2 — draft behavior for event creation
    const permission = this.checkPermission('create_calendar_event');
    if (permission.requiresApproval) {
      throw new Error(
        `Calendar createEvent requires approval: ${permission.description}. ` +
        'Event details have been logged for review.'
      );
    }

    const calendar = calendarId || this.config.defaultCalendarId || 'primary';
    await this.publishEvent('calendar.createEvent', {
      summary,
      start: start.toISOString(),
      end: end.toISOString(),
      attendees,
      calendarId: calendar,
    });

    try {
      const body: Record<string, unknown> = {
        summary,
        start: { dateTime: start.toISOString() },
        end: { dateTime: end.toISOString() },
      };

      if (attendees && attendees.length > 0) {
        body.attendees = attendees.map((email) => ({ email }));
      }

      const response = await this.apiRequest<Record<string, unknown>>(
        `https://www.googleapis.com/calendar/v3/calendars/${encodeURIComponent(calendar)}/events`,
        {
          method: 'POST',
          body: JSON.stringify(body),
        }
      );

      const event = this.parseEvent(response, calendar);

      await this.publishEvent('calendar.createEvent.complete', {
        eventId: event.id,
        summary,
        calendarId: calendar,
      });

      return event;
    } catch (err) {
      await this.publishEvent('calendar.createEvent.error', {
        summary,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async findFreeTime(timeMin: Date, timeMax: Date, durationMinutes: number): Promise<FreeTimeSlot[]> {
    this.ensureConfigured();

    const permission = this.checkPermission('query_calendar');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Calendar findFreeTime requires approval: ${permission.description}`);
    }

    await this.publishEvent('calendar.findFreeTime', {
      timeMin: timeMin.toISOString(),
      timeMax: timeMax.toISOString(),
      durationMinutes,
    });

    try {
      const events = await this.listEvents(timeMin, timeMax);

      // Sort events by start time
      events.sort((a, b) => a.start.getTime() - b.start.getTime());

      const freeSlots: FreeTimeSlot[] = [];
      let cursor = new Date(timeMin);

      for (const event of events) {
        const gapMinutes = (event.start.getTime() - cursor.getTime()) / (1000 * 60);
        if (gapMinutes >= durationMinutes) {
          freeSlots.push({
            start: new Date(cursor),
            end: new Date(event.start),
            durationMinutes: gapMinutes,
          });
        }
        if (event.end > cursor) {
          cursor = new Date(event.end);
        }
      }

      // Check gap after last event
      const finalGap = (timeMax.getTime() - cursor.getTime()) / (1000 * 60);
      if (finalGap >= durationMinutes) {
        freeSlots.push({
          start: new Date(cursor),
          end: new Date(timeMax),
          durationMinutes: finalGap,
        });
      }

      await this.publishEvent('calendar.findFreeTime.complete', {
        slotsFound: freeSlots.length,
      });

      return freeSlots;
    } catch (err) {
      await this.publishEvent('calendar.findFreeTime.error', {
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    const hasCredentials = !!(this.config.clientId && this.config.clientSecret && this.config.refreshToken);
    checks.push({
      name: 'OAuth2 credentials configured',
      passed: hasCredentials,
      message: hasCredentials
        ? 'OAuth2 credentials are present'
        : 'Missing OAuth2 credentials. Set clientId, clientSecret, and refreshToken in config.',
    });

    checks.push({
      name: 'Integration enabled',
      passed: this.config.enabled,
      message: this.config.enabled ? 'Calendar integration is enabled' : 'Calendar integration is disabled in config',
    });

    checks.push({
      name: 'API access valid',
      passed: this.tokenValid,
      message: this.tokenValid
        ? 'Google Calendar API is accessible'
        : 'API access has not been validated. Run initialize() first or check credentials.',
    });

    checks.push({
      name: 'Connection status',
      passed: this.status === 'connected',
      message: `Current status: ${this.status}`,
    });

    const allPassed = checks.every((c) => c.passed);
    const somePassed = checks.some((c) => c.passed);

    return {
      module: 'calendar',
      status: allPassed ? 'healthy' : somePassed ? 'degraded' : 'unhealthy',
      checks,
    };
  }

  private ensureConfigured(): void {
    if (!this.config.enabled) {
      throw new Error('Calendar integration is not enabled. Enable it in config.');
    }
    if (!this.config.clientId || !this.config.clientSecret || !this.config.refreshToken) {
      throw new Error('Calendar integration is not configured. Provide OAuth2 credentials.');
    }
  }

  private async validateToken(): Promise<boolean> {
    try {
      await this.apiRequest('https://www.googleapis.com/calendar/v3/users/me/calendarList?maxResults=1');
      return true;
    } catch {
      return false;
    }
  }

  private async apiRequest<T>(url: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${this.config.refreshToken}`,
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Calendar API error (${response.status}): ${errorBody}`);
    }

    return response.json() as Promise<T>;
  }

  private parseEvent(raw: Record<string, unknown>, calendarId: string): CalendarEvent {
    const start = raw.start as { dateTime?: string; date?: string } | undefined;
    const end = raw.end as { dateTime?: string; date?: string } | undefined;
    const attendees = raw.attendees as Array<{ email: string }> | undefined;

    return {
      id: raw.id as string,
      summary: (raw.summary as string) || '',
      description: raw.description as string | undefined,
      start: new Date(start?.dateTime || start?.date || ''),
      end: new Date(end?.dateTime || end?.date || ''),
      attendees: attendees?.map((a) => a.email) || [],
      calendarId,
      location: raw.location as string | undefined,
      status: (raw.status as string) || 'confirmed',
      htmlLink: raw.htmlLink as string | undefined,
    };
  }
}
