import { describe, it, expect } from "vitest";
import { listServicesTool } from "../../src/tools/list-services.js";
import { TEST_SERVICES, getText } from "../fixtures.js";

describe("list_services tool", () => {
  it("returns configured services", async () => {
    const result = await listServicesTool(TEST_SERVICES).callback();

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.count).toBe(2);
    expect(body.services).toEqual([
      { name: "go-gin", url: "http://localhost:8080" },
      { name: "python", url: "http://localhost:5000" },
    ]);
  });

  it("returns empty list when no services configured", async () => {
    const result = await listServicesTool([]).callback();

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.count).toBe(0);
    expect(body.services).toEqual([]);
  });
});
