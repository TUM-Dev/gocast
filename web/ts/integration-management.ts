import { copyToClipboard, postData } from "./global";

export interface IntegrationRow {
    id: number;
    name: string;
    returnUrl: string;
    hasKey: boolean;
}

interface CreateIntegrationResponse {
    id: number;
    name: string;
    returnUrl: string;
    apiKey: string;
}

interface IntegrationKeyResponse {
    apiKey: string;
}

export class IntegrationAdmin {
    rows: IntegrationRow[];
    name = "";
    returnUrl = "";
    generatedKey = "";
    generatedId = 0;
    error = "";
    copied = "";
    busy = false;

    constructor(rows: IntegrationRow[]) {
        this.rows = rows;
    }

    private beginRequest() {
        this.error = "";
        this.copied = "";
        this.busy = true;
    }

    private showKey(row: IntegrationRow, key: string) {
        this.generatedId = row.id;
        this.generatedKey = key;
        this.copied = "";
    }

    async create() {
        this.beginRequest();
        try {
            const response = await postData("/api/integrations", { name: this.name, returnUrl: this.returnUrl });
            if (!response.ok) {
                const error: { message?: string } | null = await response.json().catch(() => null);
                this.error = error?.message || `The request failed (${response.status}). Try again.`;
                return;
            }
            const body: CreateIntegrationResponse | null = await response.json().catch(() => null);
            if (!body || body.id === undefined || !body.name || !body.returnUrl || !body.apiKey) {
                this.error = "The server returned an unreadable response. Try again.";
                return;
            }
            const row = { id: body.id, name: body.name, returnUrl: body.returnUrl, hasKey: true };
            this.rows.push(row);
            this.showKey(row, body.apiKey);
            this.name = "";
            this.returnUrl = "";
        } catch (_) {
            this.error = "The request could not reach the server. Try again.";
        } finally {
            this.busy = false;
        }
    }

    async rotate(row: IntegrationRow) {
        this.beginRequest();
        try {
            const response = await postData(`/api/integrations/${row.id}/key`);
            if (!response.ok) {
                const error: { message?: string } | null = await response.json().catch(() => null);
                this.error = error?.message || `The request failed (${response.status}). Try again.`;
                return;
            }
            const body: IntegrationKeyResponse | null = await response.json().catch(() => null);
            if (!body?.apiKey) {
                this.error = "The server returned an unreadable response. Try again.";
                return;
            }
            row.hasKey = true;
            this.showKey(row, body.apiKey);
        } catch (_) {
            this.error = "The request could not reach the server. Try again.";
        } finally {
            this.busy = false;
        }
    }

    async revoke(row: IntegrationRow) {
        this.beginRequest();
        try {
            const response = await fetch(`/api/integrations/${row.id}/key`, { method: "DELETE" });
            if (!response.ok) {
                const error: { message?: string } | null = await response.json().catch(() => null);
                this.error = error?.message || `The request failed (${response.status}). Try again.`;
                return;
            }
            row.hasKey = false;
            if (this.generatedId === row.id) {
                this.showKey(row, "");
            }
        } catch (_) {
            this.error = "The request could not reach the server. Try again.";
        } finally {
            this.busy = false;
        }
    }

    async copy() {
        let copied = false;
        try {
            copied = await copyToClipboard(this.generatedKey);
        } catch (_) {
            copied = false;
        }
        this.copied = copied ? "Copied." : "Copy failed. Select the key and copy it manually.";
    }
}
