/**
 * CLI output formatting utilities
 */

const COLORS = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
};

export function green(text: string): string {
  return `${COLORS.green}${text}${COLORS.reset}`;
}

export function red(text: string): string {
  return `${COLORS.red}${text}${COLORS.reset}`;
}

export function yellow(text: string): string {
  return `${COLORS.yellow}${text}${COLORS.reset}`;
}

export function blue(text: string): string {
  return `${COLORS.blue}${text}${COLORS.reset}`;
}

export function cyan(text: string): string {
  return `${COLORS.cyan}${text}${COLORS.reset}`;
}

export function dim(text: string): string {
  return `${COLORS.dim}${text}${COLORS.reset}`;
}

export function bold(text: string): string {
  return `${COLORS.bold}${text}${COLORS.reset}`;
}

export function statusIcon(status: string): string {
  switch (status) {
    case 'healthy':
    case 'running':
    case 'completed':
      return green('✓');
    case 'degraded':
    case 'paused':
    case 'needs_review':
    case 'blocked':
      return yellow('⚠');
    case 'unhealthy':
    case 'error':
    case 'failed':
    case 'stopped':
      return red('✗');
    default:
      return dim('○');
  }
}

export function statusColor(status: string): string {
  switch (status) {
    case 'healthy':
    case 'running':
    case 'completed':
      return green(status);
    case 'degraded':
    case 'paused':
    case 'needs_review':
    case 'blocked':
      return yellow(status);
    case 'unhealthy':
    case 'error':
    case 'failed':
    case 'stopped':
      return red(status);
    default:
      return dim(status);
  }
}

export function table(rows: string[][], columnWidths?: number[]): string {
  if (rows.length === 0) return '';

  const widths = columnWidths ?? rows[0].map((_, i) =>
    Math.max(...rows.map((r) => stripAnsi(r[i] ?? '').length))
  );

  return rows.map((row) =>
    row.map((cell, i) => {
      const stripped = stripAnsi(cell);
      const padding = Math.max(0, (widths[i] ?? 0) - stripped.length);
      return cell + ' '.repeat(padding);
    }).join('  ')
  ).join('\n');
}

function stripAnsi(str: string): string {
  return str.replace(/\x1b\[\d+m/g, '');
}

export function timeAgo(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}
