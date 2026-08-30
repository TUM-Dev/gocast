import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import LoginView from "./LoginView.vue";

/**
 * The login page's job is to offer the right controls before anyone is authenticated,
 * and to hand credentials to Go rather than to the API. These tests cover both.
 */

const { fetchLoginOptions, requestPasswordReset } = vi.hoisted(() => ({
  fetchLoginOptions: vi.fn(),
  requestPasswordReset: vi.fn(),
}));

vi.mock("@/lib/login", () => ({ fetchLoginOptions, requestPasswordReset }));

// LoginView reads the query through useRoute, so the router itself is stubbed.
vi.mock("vue-router", () => ({ useRoute: () => mockRoute }));
let mockRoute: { query: Record<string, string> } = { query: {} };

function mountLogin() {
  return mount(LoginView);
}

beforeEach(() => {
  vi.clearAllMocks();
  mockRoute = { query: {} };
  fetchLoginOptions.mockResolvedValue({ useSaml: false, idpName: "", idpColor: "#3070B3" });
});

describe("LoginView", () => {
  it("posts credentials to Go rather than to the API", async () => {
    // The server has to set the session cookie and honour the stored redirect, so this
    // stays a real form submission.
    const wrapper = mountLogin();
    await flushPromises();

    const form = wrapper.get("#loginForm");
    expect(form.attributes("method")).toBe("post");
    expect(form.attributes("action")).toBe("/login");
    expect(wrapper.find("input[name=username]").exists()).toBe(true);
    expect(wrapper.get("input[name=password]").attributes("type")).toBe("password");
  });

  it("shows the identity provider button when single sign-on is configured", async () => {
    fetchLoginOptions.mockResolvedValue({
      useSaml: true,
      idpName: "TUM Login",
      idpColor: "#abcdef",
    });

    const wrapper = mountLogin();
    await flushPromises();

    const sso = wrapper.get('a[href="/saml/out"]');
    expect(sso.text()).toBe("TUM Login");
    // The internal form stays one click away rather than competing with SSO.
    expect(wrapper.find("#loginForm").exists()).toBe(false);
  });

  it("offers the internal form when single sign-on is not configured", async () => {
    const wrapper = mountLogin();
    await flushPromises();

    expect(wrapper.find('a[href="/saml/out"]').exists()).toBe(false);
    expect(wrapper.find("#loginForm").exists()).toBe(true);
  });

  it("still offers a way in when the options cannot be loaded", async () => {
    fetchLoginOptions.mockRejectedValue(new Error("network"));

    const wrapper = mountLogin();
    await flushPromises();

    expect(wrapper.find("#loginForm").exists()).toBe(true);
  });

  it("reports a failed attempt from the redirect", async () => {
    mockRoute = { query: { error: "" } };

    const wrapper = mountLogin();
    await flushPromises();

    expect(wrapper.text()).toContain("Couldn't log in");
  });

  it("does not claim failure on a first visit", async () => {
    const wrapper = mountLogin();
    await flushPromises();

    expect(wrapper.text()).not.toContain("Couldn't log in");
  });

  it("confirms a password reset without revealing whether the account exists", async () => {
    requestPasswordReset.mockResolvedValue(undefined);

    const wrapper = mountLogin();
    await flushPromises();

    await wrapper.get("button.text-5").trigger("click");
    await wrapper.get("#reset-email").setValue("someone@example.org");
    await wrapper.get("form").trigger("submit");
    await flushPromises();

    expect(requestPasswordReset).toHaveBeenCalledWith("someone@example.org");
    expect(wrapper.text()).toContain("if the username you provided is associated with an account");
  });
});
