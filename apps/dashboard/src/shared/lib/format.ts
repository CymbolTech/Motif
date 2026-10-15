/** One timestamp vocabulary for the console: a clock time for dense tables, a full stamp everywhere else. */
export function formatTime(value: string) {
  return new Date(value).toLocaleTimeString([], { timeStyle: 'medium' });
}

export function formatDateTime(value: string, options: { seconds?: boolean } = {}) {
  return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: options.seconds ? 'medium' : 'short' });
}
