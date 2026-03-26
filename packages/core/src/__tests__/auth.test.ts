import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  signCookie,
  verifyCookie,
  isEmailAllowed,
  parseCookies,
  getSessionEmail,
  buildGoogleAuthUrl,
  loadAuthConfig,
} from '../server/auth.js';

describe('signCookie / verifyCookie', () => {
  const SECRET = 'test-secret-abc123';
  const EMAIL = 'tobiasgunn@gmail.com';

  it('round-trips: sign then verify returns the email', () => {
    const value = signCookie(EMAIL, SECRET);
    const result = verifyCookie(value, SECRET);
    expect(result).toBe(EMAIL);
  });

  it('returns null for a tampered signature', () => {
    const value = signCookie(EMAIL, SECRET);
    const tampered = value.slice(0, -4) + 'xxxx';
    expect(verifyCookie(tampered, SECRET)).toBeNull();
  });

  it('returns null for a wrong secret', () => {
    const value = signCookie(EMAIL, SECRET);
    expect(verifyCookie(value, 'wrong-secret')).toBeNull();
  });

  it('returns null for an expired cookie', () => {
    // Fake a timestamp 8 days in the past
    const eightDaysAgo = (Date.now() - 8 * 24 * 60 * 60 * 1000).toString();
    // Build a valid-looking but expired payload manually
    const { createHmac } = require('node:crypto');
    const data = `v1:${EMAIL}:${eightDaysAgo}`;
    const sig = createHmac('sha256', SECRET).update(data).digest('hex');
    const value = `${data}:${sig}`;
    expect(verifyCookie(value, SECRET)).toBeNull();
  });

  it('returns null for a completely invalid string', () => {
    expect(verifyCookie('garbage', SECRET)).toBeNull();
    expect(verifyCookie('', SECRET)).toBeNull();
    expect(verifyCookie('v1:only:two', SECRET)).toBeNull();
  });

  it('handles email addresses with subdomains correctly', () => {
    const email = 'user@sub.domain.com';
    const value = signCookie(email, SECRET);
    expect(verifyCookie(value, SECRET)).toBe(email);
  });
});

describe('isEmailAllowed', () => {
  const ALLOWED = ['tobiasgunn@gmail.com', 'admin@fort.io'];

  it('returns true for an allowed email (exact match)', () => {
    expect(isEmailAllowed('tobiasgunn@gmail.com', ALLOWED)).toBe(true);
  });

  it('is case-insensitive', () => {
    expect(isEmailAllowed('TobiasGunn@Gmail.COM', ALLOWED)).toBe(true);
    expect(isEmailAllowed('ADMIN@FORT.IO', ALLOWED)).toBe(true);
  });

  it('returns false for an email not in the allowlist', () => {
    expect(isEmailAllowed('hacker@evil.com', ALLOWED)).toBe(false);
    expect(isEmailAllowed('', ALLOWED)).toBe(false);
  });

  it('returns false for an empty allowlist', () => {
    expect(isEmailAllowed('tobiasgunn@gmail.com', [])).toBe(false);
  });
});

describe('parseCookies', () => {
  it('parses a single cookie', () => {
    const cookies = parseCookies('fort_session=abc123');
    expect(cookies['fort_session']).toBe('abc123');
  });

  it('parses multiple cookies', () => {
    const cookies = parseCookies('a=1; b=2; c=3');
    expect(cookies['a']).toBe('1');
    expect(cookies['b']).toBe('2');
    expect(cookies['c']).toBe('3');
  });

  it('decodes URI-encoded values', () => {
    const cookies = parseCookies('fort_session=hello%3Aworld');
    expect(cookies['fort_session']).toBe('hello:world');
  });

  it('returns empty object for undefined', () => {
    expect(parseCookies(undefined)).toEqual({});
  });

  it('returns empty object for empty string', () => {
    expect(parseCookies('')).toEqual({});
  });
});

describe('getSessionEmail', () => {
  const SECRET = 'session-test-secret';
  const EMAIL = 'tobiasgunn@gmail.com';

  it('returns email for a valid session cookie', () => {
    const { encodeURIComponent: encode } = global;
    const value = signCookie(EMAIL, SECRET);
    const cookieHeader = `fort_session=${encode(value)}`;
    expect(getSessionEmail(cookieHeader, SECRET)).toBe(EMAIL);
  });

  it('returns null when fort_session cookie is missing', () => {
    expect(getSessionEmail('other_cookie=foo', SECRET)).toBeNull();
    expect(getSessionEmail(undefined, SECRET)).toBeNull();
  });

  it('returns null for a tampered cookie', () => {
    const value = signCookie(EMAIL, SECRET);
    const tampered = value.slice(0, -4) + '0000';
    const cookieHeader = `fort_session=${encodeURIComponent(tampered)}`;
    expect(getSessionEmail(cookieHeader, SECRET)).toBeNull();
  });
});

describe('buildGoogleAuthUrl', () => {
  it('includes required OAuth2 params', () => {
    const url = buildGoogleAuthUrl('my-client-id', 'http://localhost:4077/auth/google/callback');
    const parsed = new URL(url);
    expect(parsed.hostname).toBe('accounts.google.com');
    expect(parsed.searchParams.get('client_id')).toBe('my-client-id');
    expect(parsed.searchParams.get('response_type')).toBe('code');
    expect(parsed.searchParams.get('scope')).toContain('email');
    expect(parsed.searchParams.get('redirect_uri')).toBe(
      'http://localhost:4077/auth/google/callback',
    );
  });
});

describe('loadAuthConfig', () => {
  const ORIGINAL_ENV = { ...process.env };

  afterEach(() => {
    // Restore env after each test
    for (const key of [
      'GOOGLE_CLIENT_ID',
      'GOOGLE_CLIENT_SECRET',
      'SESSION_SECRET',
      'FORT_ALLOWED_EMAILS',
      'GOOGLE_CALLBACK_URL',
      'FORT_AUTH_ENABLED',
    ]) {
      if (ORIGINAL_ENV[key] !== undefined) {
        process.env[key] = ORIGINAL_ENV[key];
      } else {
        delete process.env[key];
      }
    }
  });

  it('uses defaults when env vars are absent', () => {
    delete process.env.FORT_ALLOWED_EMAILS;
    delete process.env.FORT_AUTH_ENABLED;
    const config = loadAuthConfig();
    expect(config.allowedEmails).toContain('tobiasgunn@gmail.com');
    expect(config.authEnabled).toBe(true);
  });

  it('parses comma-separated FORT_ALLOWED_EMAILS', () => {
    process.env.FORT_ALLOWED_EMAILS = 'alice@example.com, bob@example.com';
    const config = loadAuthConfig();
    expect(config.allowedEmails).toContain('alice@example.com');
    expect(config.allowedEmails).toContain('bob@example.com');
  });

  it('disables auth when FORT_AUTH_ENABLED=false', () => {
    process.env.FORT_AUTH_ENABLED = 'false';
    const config = loadAuthConfig();
    expect(config.authEnabled).toBe(false);
  });
});
