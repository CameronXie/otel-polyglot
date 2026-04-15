import type { ServiceEntry } from "../src/config.js";

export const TEST_SERVICES: ServiceEntry[] = [
  { name: "go-gin", url: "http://localhost:8080" },
  { name: "python", url: "http://localhost:5000" },
];

/** Extract text from the first content item of a CallToolResult. */
export function getText(
  result: { content: Array<{ type: string; text?: string }> },
  index = 0,
): string {
  const item = result.content[index];
  if (!item || item.type !== "text" || typeof item.text !== "string") {
    throw new Error(
      `Expected text content at index ${index}, got ${item?.type ?? "undefined"}`,
    );
  }
  return item.text;
}
