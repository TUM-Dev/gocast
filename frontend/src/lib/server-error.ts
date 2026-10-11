/**
 * The error Go refused the page request with. It answers with the shell and the
 * status, and names the problem in two meta tags (web.serveErrorPage); without them
 * the shell would render whatever page the URL names for a request the server just
 * refused. Absent on a normal load.
 */
export interface ServerError {
  status: number;
  message: string;
}

export function readServerError(doc: Document = document): ServerError | null {
  const status = Number(doc.querySelector('meta[name="gocast-error-status"]')?.getAttribute("content"));
  if (!Number.isInteger(status) || status < 400) return null;
  return {
    status,
    message: doc.querySelector('meta[name="gocast-error-message"]')?.getAttribute("content") ?? "",
  };
}
