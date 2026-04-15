import { ToolBuilder, jsonResult } from "./tools.js";
import type { ServiceEntry } from "../config.js";

export const listServicesTool = (services: ServiceEntry[]) =>
  new ToolBuilder(
    "list_services",
    "List all configured backend services and their URLs",
  ).handler(async () => {
    return jsonResult({
      services: services.map((s) => ({ name: s.name, url: s.url })),
      count: services.length,
    });
  });
