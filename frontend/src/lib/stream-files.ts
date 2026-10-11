/**
 * The URL of a file of a stream (thumbnail, section image, attachment, recording
 * download): GET /api/v2/streams/{stream}/files/{file}.
 *
 * Served with the same access check as watching the stream, so it works in an <img>
 * or a link on the page's own cookie. `download` has the browser save the file
 * instead of showing it.
 */
export function streamFileUrl(streamID: number, fileID: number, options: { download?: boolean } = {}): string {
  const url = `/api/v2/streams/${streamID}/files/${fileID}`;
  return options.download ? `${url}?download=1` : url;
}
