#!/usr/bin/env bun
import { password } from "bun";

const input = process.argv[2] || await prompt("Enter password: ");

if (!input) {
  console.error("Error: Password cannot be empty");
  process.exit(1);
}

const hash = await password.hash(input, { algorithm: "bcrypt", cost: 10 });

console.log("\nAdd this to your .env file:");
console.log(`AUTH_PASSWORD_HASH='${hash}'`);

async function prompt(message: string): Promise<string> {
  process.stdout.write(message);
  for await (const line of console) {
    return line.trim();
  }
  return "";
}
