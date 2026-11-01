// Prints every screenshot in a folder into the CI log as base64, between marker lines, so the images
// can be read back from the job log for a pull request without artifact storage.
//
//   node design/screens/print-screens.mjs <folder>
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const folder = process.argv[2];
for (const name of readdirSync(folder).filter(file => /\.(jpg|png)$/.test(file)).sort()) {
  const data = readFileSync(join(folder, name)).toString('base64');
  console.log(`==== fort-screen ${name} ${data.length} ====`);
  for (let start = 0; start < data.length; start += 1000) console.log(data.slice(start, start + 1000));
  console.log(`==== fort-screen-end ${name} ====`);
}
