import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const envPath = resolve(projectRoot, '.env');
const content = readFileSync(envPath, 'utf8');

for (const line of content.split(/\r?\n/)) {
  const trimmed = line.trim();
  if (!trimmed || trimmed.startsWith('#')) continue;

  const separator = trimmed.indexOf('=');
  if (separator === -1) continue;

  const key = trimmed.slice(0, separator).trim();
  let value = trimmed.slice(separator + 1).trim();
  if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
    value = value.slice(1, -1);
  }

  value = value.replace(/\\n/g, '\n');
  console.log(`Syncing ${key}...`);

  try {
    execFileSync('vercel', ['env', 'rm', key, 'production', '-y', '--non-interactive'], {
      cwd: projectRoot,
      stdio: 'ignore',
    });
  } catch {
    // The variable may not exist yet.
  }

  try {
    execFileSync('vercel', ['env', 'add', key, 'production', '--non-interactive'], {
      cwd: projectRoot,
      input: value,
      stdio: ['pipe', 'inherit', 'inherit'],
    });
    console.log(`Successfully added ${key}`);
  } catch (error) {
    console.error(`Failed to add ${key}:`, error.message);
  }
}
