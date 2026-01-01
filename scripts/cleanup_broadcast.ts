#!/usr/bin/env bun
/**
 * Cleanup broadcast status entries (status@broadcast) from chat storage.
 * Uses the new admin storage API.
 *
 * Environment:
 *   WA_USER - Basic auth username
 *   WA_PASS - Basic auth password
 *
 * Usage:
 *   WA_USER=x WA_PASS=y bun run cleanup_broadcast.ts b100
 *   WA_USER=x WA_PASS=y bun run cleanup_broadcast.ts --all
 *   WA_USER=x WA_PASS=y bun run cleanup_broadcast.ts b100 --dry-run
 */

import { parseArgs } from "util";

// Get credentials from environment variables
const USER = process.env.WA_USER;
const PASS = process.env.WA_PASS;
if (!USER || !PASS) {
  console.error("Set WA_USER and WA_PASS environment variables");
  process.exit(1);
}
const AUTH_HEADER = Buffer.from(`${USER}:${PASS}`).toString("base64");

interface CleanupResult {
  chats_deleted: number;
  messages_deleted: number;
  dry_run: boolean;
}

interface ApiResponse {
  code: string;
  message: string;
  results: CleanupResult;
}

async function isDeviceConnected(host: string): Promise<boolean> {
  const url = `https://${host}.bnis.my.id/app/devices`;
  try {
    const response = await fetch(url, {
      headers: { Authorization: `Basic ${AUTH_HEADER}` },
    });
    if (!response.ok) return false;
    const data = await response.json();
    return Array.isArray(data.results) && data.results.length > 0;
  } catch {
    return false;
  }
}

async function cleanupBroadcast(host: string, dryRun: boolean): Promise<CleanupResult | null> {
  const url = `https://${host}.bnis.my.id/admin/storage/cleanup`;
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Basic ${AUTH_HEADER}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        pattern: "%@broadcast",
        dry_run: dryRun,
      }),
    });

    if (!response.ok) {
      const text = await response.text();
      console.error(`API error: ${response.status} - ${text}`);
      return null;
    }

    const data: ApiResponse = await response.json();
    return data.results;
  } catch (err) {
    console.error(`Request failed: ${err}`);
    return null;
  }
}

async function processHost(host: string, dryRun: boolean): Promise<boolean> {
  process.stdout.write(`${host}: `);

  // Check if device is connected
  if (!await isDeviceConnected(host)) {
    console.log("skipped (not connected)");
    return true;
  }

  const result = await cleanupBroadcast(host, dryRun);
  if (!result) {
    console.log("FAILED");
    return false;
  }

  if (result.chats_deleted === 0) {
    console.log("no broadcast chats found");
    return true;
  }

  if (dryRun) {
    console.log(`would delete ${result.chats_deleted} chats, ${result.messages_deleted} messages`);
  } else {
    console.log(`deleted ${result.chats_deleted} chats, ${result.messages_deleted} messages`);
  }

  return true;
}

async function main() {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      "dry-run": { type: "boolean", default: false },
      all: { type: "boolean", default: false },
    },
    allowPositionals: true,
  });

  let hosts: string[] = [];

  if (values.all) {
    hosts = Array.from({ length: 100 }, (_, i) => `b${i + 1}`);
  } else if (positionals.length > 0) {
    hosts = positionals;
  } else {
    console.log("Usage:");
    console.log("  bun run cleanup_broadcast.ts b100           # Clean single host");
    console.log("  bun run cleanup_broadcast.ts b1 b2 b3       # Clean multiple hosts");
    console.log("  bun run cleanup_broadcast.ts --all          # Clean all b1-b100");
    console.log("  bun run cleanup_broadcast.ts b100 --dry-run # Preview only");
    process.exit(1);
  }

  console.log(`\n${"=".repeat(50)}`);
  console.log(`Broadcast Cleanup ${values["dry-run"] ? "(DRY RUN)" : ""}`);
  console.log(`${"=".repeat(50)}\n`);

  let success = 0,
    failed = 0;
  for (const host of hosts) {
    if (await processHost(host, values["dry-run"] || false)) {
      success++;
    } else {
      failed++;
    }
  }

  console.log(`\nDone: ${success} success, ${failed} failed`);
}

main().catch(console.error);
