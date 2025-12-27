#!/usr/bin/env bun
/**
 * Fix LID/JID duplicate entries in WhatsApp chat storage.
 *
 * Environment:
 *   WA_USER - Basic auth username
 *   WA_PASS - Basic auth password
 *
 * Usage:
 *   WA_USER=x WA_PASS=y bun run fix_lid_duplicates.ts b100
 *   WA_USER=x WA_PASS=y bun run fix_lid_duplicates.ts --all
 *   WA_USER=x WA_PASS=y bun run fix_lid_duplicates.ts b100 --dry-run
 */

import { Database } from "bun:sqlite";
import { parseArgs } from "util";
import { unlinkSync, copyFileSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";

// Get credentials from environment variables
const USER = process.env.WA_USER;
const PASS = process.env.WA_PASS;
if (!USER || !PASS) {
  console.error("Set WA_USER and WA_PASS environment variables");
  process.exit(1);
}
const AUTH_HEADER = Buffer.from(`${USER}:${PASS}`).toString("base64");

// Known country codes for LID detection
const KNOWN_COUNTRY_CODES = new Set([
  "1", "7", "20", "27", "30", "31", "32", "33", "34", "36", "39", "40", "41",
  "43", "44", "45", "46", "47", "48", "49", "51", "52", "53", "54", "55", "56",
  "57", "58", "60", "61", "62", "63", "64", "65", "66", "81", "82", "84", "86",
  "90", "91", "92", "93", "94", "95", "98"
]);

interface ChatInfo {
  jid: string;
  name: string;
  user: string;
  message_count?: number;
}

function isLikelyLID(jidUser: string): boolean {
  if (!/^\d+$/.test(jidUser)) return false;
  if (jidUser.length > 15) return true;
  for (const codeLen of [1, 2, 3]) {
    if (jidUser.length >= codeLen) {
      const prefix = jidUser.substring(0, codeLen);
      if (KNOWN_COUNTRY_CODES.has(prefix)) return false;
    }
  }
  return jidUser.length >= 12;
}

async function isDeviceConnected(host: string): Promise<boolean> {
  const url = `https://${host}.bnis.my.id/app/devices`;
  const proc = Bun.spawn([
    "curl", "-s", "--location",
    "-H", `Authorization: Basic ${AUTH_HEADER}`,
    url
  ], { stdout: "pipe", stderr: "pipe" });

  const stdout = await new Response(proc.stdout).text();
  await proc.exited;

  try {
    const data = JSON.parse(stdout);
    return Array.isArray(data.results) && data.results.length > 0;
  } catch {
    return false;
  }
}

async function downloadBackup(host: string, outputPath: string): Promise<boolean> {
  const url = `https://${host}.bnis.my.id/chat/export`;
  const proc = Bun.spawn([
    "curl", "-s", "-f", "--location",
    "-H", `Authorization: Basic ${AUTH_HEADER}`,
    url, "-o", outputPath
  ], { stdout: "pipe", stderr: "pipe" });
  await proc.exited;
  return proc.exitCode === 0;
}

async function uploadBackup(host: string, filePath: string): Promise<boolean> {
  const url = `https://${host}.bnis.my.id/chat/import`;
  const proc = Bun.spawn([
    "curl", "-s", "-f", "--location", "-X", "POST",
    "-H", `Authorization: Basic ${AUTH_HEADER}`,
    "-F", `file=@${filePath}`,
    "-F", "overwrite=true",
    url
  ], { stdout: "pipe", stderr: "pipe" });
  await proc.exited;
  return proc.exitCode === 0;
}

function analyzeAndFix(dbPath: string, dryRun: boolean): { lids: number; fixed: number; messages: number; error?: string } {
  const db = new Database(dbPath);
  const stats = { lids: 0, fixed: 0, messages: 0 };

  // Check if chats table exists
  const tableCheck = db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='chats'").get();
  if (!tableCheck) {
    db.close();
    return { ...stats, error: "no data" };
  }

  // Find potential LIDs (both @lid and @s.whatsapp.net with LID-like numbers)
  const chats = db.query("SELECT jid, name FROM chats").all() as any[];
  const lids: ChatInfo[] = [];
  const normalJids: ChatInfo[] = [];

  for (const chat of chats) {
    const jid = chat.jid as string;
    if (jid.includes("@g.us")) continue;

    const user = jid.split("@")[0];
    if (isLikelyLID(user)) {
      lids.push({ jid, name: chat.name, user });
    } else {
      normalJids.push({ jid, name: chat.name, user });
    }
  }

  stats.lids = lids.length;

  // Auto-fix by matching names
  for (const lid of lids) {
    const match = normalJids.find(n => lid.name && lid.name !== lid.user && n.name === lid.name);
    if (!match) continue;

    // Count and move messages
    const messages = db.query("SELECT id FROM messages WHERE chat_jid = ?").all(lid.jid) as any[];
    let moved = 0;

    for (const msg of messages) {
      const exists = db.query("SELECT 1 FROM messages WHERE id = ? AND chat_jid = ?").get(msg.id, match.jid);
      if (exists) continue;

      if (!dryRun) {
        db.query("UPDATE messages SET chat_jid = ? WHERE id = ? AND chat_jid = ?").run(match.jid, msg.id, lid.jid);
      }
      moved++;
    }

    // Delete LID chat
    if (!dryRun) {
      db.query("DELETE FROM messages WHERE chat_jid = ?").run(lid.jid);
      db.query("DELETE FROM chats WHERE jid = ?").run(lid.jid);
    }

    if (moved > 0 || messages.length > 0) {
      console.log(`  ${lid.jid} -> ${match.jid} (${moved} messages)`);
      stats.fixed++;
      stats.messages += moved;
    }
  }

  db.close();
  return stats;
}

async function fixHost(host: string, dryRun: boolean): Promise<boolean> {
  const tempDir = tmpdir();
  const backupPath = join(tempDir, `${host}-backup-${Date.now()}.db`);
  const fixedPath = join(tempDir, `${host}-fixed-${Date.now()}.db`);

  try {
    process.stdout.write(`${host}: `);

    // Check if device is connected
    if (!await isDeviceConnected(host)) {
      console.log("skipped (not connected)");
      return true; // Not a failure, just skip
    }

    process.stdout.write(`downloading... `);
    if (!await downloadBackup(host, backupPath)) {
      console.log("FAILED (download)");
      return false;
    }

    copyFileSync(backupPath, fixedPath);
    const stats = analyzeAndFix(fixedPath, dryRun);

    if (stats.error) {
      console.log(stats.error);
      return true;
    }

    if (stats.lids === 0) {
      console.log("no LIDs found");
      return true;
    }

    if (stats.fixed === 0) {
      console.log(`${stats.lids} LIDs (no auto-match)`);
      return true;
    }

    if (dryRun) {
      console.log(`would fix ${stats.fixed}/${stats.lids} LIDs (${stats.messages} messages)`);
      return true;
    }

    process.stdout.write(`uploading... `);
    if (!await uploadBackup(host, fixedPath)) {
      console.log("FAILED (upload)");
      return false;
    }

    console.log(`fixed ${stats.fixed}/${stats.lids} LIDs (${stats.messages} messages)`);
    return true;

  } finally {
    try { unlinkSync(backupPath); } catch {}
    try { unlinkSync(fixedPath); } catch {}
  }
}

async function main() {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      "dry-run": { type: "boolean", default: false },
      "all": { type: "boolean", default: false },
    },
    allowPositionals: true,
  });

  let hosts: string[] = [];

  if (values["all"]) {
    hosts = Array.from({ length: 100 }, (_, i) => `b${i + 1}`);
  } else if (positionals.length > 0) {
    hosts = positionals;
  } else {
    console.log("Usage:");
    console.log("  bun run fix_lid_duplicates.ts b100           # Fix single host");
    console.log("  bun run fix_lid_duplicates.ts b1 b2 b3       # Fix multiple hosts");
    console.log("  bun run fix_lid_duplicates.ts --all          # Fix all b1-b100");
    console.log("  bun run fix_lid_duplicates.ts b100 --dry-run # Analyze only");
    process.exit(1);
  }

  console.log(`\n${"=".repeat(50)}`);
  console.log(`LID Fixer ${values["dry-run"] ? "(DRY RUN)" : ""}`);
  console.log(`${"=".repeat(50)}\n`);

  let success = 0, failed = 0;
  for (const host of hosts) {
    if (await fixHost(host, values["dry-run"] || false)) {
      success++;
    } else {
      failed++;
    }
  }

  console.log(`\nDone: ${success} success, ${failed} failed`);
}

main().catch(console.error);
