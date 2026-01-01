#!/usr/bin/env bun
/**
 * Run SQLite VACUUM on chat storage databases to optimize and reclaim space.
 * Uses the admin storage API.
 *
 * Environment:
 *   WA_USER - Basic auth username
 *   WA_PASS - Basic auth password
 *
 * Usage:
 *   WA_USER=x WA_PASS=y bun run vacuum_database.ts b100
 *   WA_USER=x WA_PASS=y bun run vacuum_database.ts --all
 *   WA_USER=x WA_PASS=y bun run vacuum_database.ts b1 b2 b3
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

interface VacuumResult {
  size_before: string;
  size_after: string;
  reclaimed: string;
}

interface ApiResponse {
  code: string;
  message: string;
  results: VacuumResult;
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

async function runVacuum(host: string): Promise<VacuumResult | null> {
  const url = `https://${host}.bnis.my.id/admin/storage/vacuum`;
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Basic ${AUTH_HEADER}`,
        "Content-Type": "application/json",
      },
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

async function processHost(host: string): Promise<{ success: boolean; reclaimed: string }> {
  process.stdout.write(`${host}: `);

  // Check if device is connected
  if (!await isDeviceConnected(host)) {
    console.log("skipped (not connected)");
    return { success: true, reclaimed: "0 B" };
  }

  const result = await runVacuum(host);
  if (!result) {
    console.log("FAILED");
    return { success: false, reclaimed: "0 B" };
  }

  console.log(`${result.size_before} -> ${result.size_after} (reclaimed: ${result.reclaimed})`);
  return { success: true, reclaimed: result.reclaimed };
}

async function main() {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
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
    console.log("  bun run vacuum_database.ts b100       # Vacuum single host");
    console.log("  bun run vacuum_database.ts b1 b2 b3   # Vacuum multiple hosts");
    console.log("  bun run vacuum_database.ts --all      # Vacuum all b1-b100");
    process.exit(1);
  }

  console.log(`\n${"=".repeat(50)}`);
  console.log(`Database Vacuum`);
  console.log(`${"=".repeat(50)}\n`);

  let success = 0,
    failed = 0;
  for (const host of hosts) {
    const result = await processHost(host);
    if (result.success) {
      success++;
    } else {
      failed++;
    }
  }

  console.log(`\nDone: ${success} success, ${failed} failed`);
}

main().catch(console.error);
