import { beforeEach, describe, expect, it, vi } from "vitest";

const { get, post, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
}));

vi.mock("../client", () => ({
  apiClient: {
    get,
    post,
    delete: del,
  },
}));

import {
  addSiteIPBlacklist,
  getSiteIPBlacklist,
  removeSiteIPBlacklist,
} from "@/api/admin/settings";

describe("admin settings site IP blacklist API", () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
    del.mockReset();
  });

  it("loads the backend items response and normalizes it for the view", async () => {
    get.mockResolvedValue({ data: { items: ["47.92.239.70"] } });

    await expect(getSiteIPBlacklist()).resolves.toEqual({
      ips: ["47.92.239.70"],
    });
    expect(get).toHaveBeenCalledWith("/admin/settings/ip-blacklist");
  });

  it("posts an IP/CIDR entry", async () => {
    post.mockResolvedValue({ data: { items: ["203.0.113.0/24"] } });

    await expect(addSiteIPBlacklist({ ip: "203.0.113.0/24" })).resolves.toEqual({
      ips: ["203.0.113.0/24"],
    });
    expect(post).toHaveBeenCalledWith("/admin/settings/ip-blacklist", {
      ip: "203.0.113.0/24",
    });
  });

  it("uses the legacy-compatible DELETE body form for IPv4 and CIDR values", async () => {
    del.mockResolvedValue({ data: { items: [] } });

    await expect(removeSiteIPBlacklist("203.0.113.0/24")).resolves.toEqual({
      ips: [],
    });
    expect(del).toHaveBeenCalledWith("/admin/settings/ip-blacklist", {
      data: { ip: "203.0.113.0/24" },
    });
  });
});
